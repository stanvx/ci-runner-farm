package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// defaultEnginePath is the install path of the bash engine. CRF_ENGINE overrides
// it for tests and for deploy.sh iteration against a live host.
const defaultEnginePath = "/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh"

// The engine speaks exactly three exit codes to the daemon (contract section 2).
// Anything else is treated as fatal, because an unmapped code means the engine
// changed underneath us and guessing "probably transient" would spin forever.
const (
	exitBusy  = 75 // EX_TEMPFAIL: fleet lock timed out, or host/fleet at capacity
	exitFatal = 1
)

var (
	// errEngineBusy is back-pressure, not a fault. Retry with backoff.
	errEngineBusy = errors.New("engine busy")
	// errEngineFatal means stop provisioning and mark the fleet unhealthy.
	errEngineFatal = errors.New("engine hard failure")
)

// Engine serialises every call to runner-farm.sh for one fleet. GitHub can ask
// for ten runners in a single message; ten concurrent jit-starts would all pile
// onto the same `flock -w 20` fleet lock and blow the timeout, turning a normal
// burst into a wave of exit 75. Queueing here means the fleet lock is only ever
// contested by a UI action, which is what it was sized for.
//
// ponytail: one slot, which is what the contract asks for. A channel rather than
// a sync.Mutex so a caller whose own context expires stops waiting instead of
// piling up behind a stuck subprocess with nothing to notice. Upgrade path: a
// second slot for the read-only verbs if jit-list latency behind a slow
// jit-start ever matters.
type Engine struct {
	path  string
	fleet string
	log   *slog.Logger
	sem   chan struct{}
}

func NewEngine(fleet string, log *slog.Logger) *Engine {
	path := os.Getenv("CRF_ENGINE")
	if path == "" {
		path = defaultEnginePath
	}
	return &Engine{path: path, fleet: fleet, log: log, sem: make(chan struct{}, 1)}
}

const (
	// engineCallTimeout bounds one engine call. Without a deadline a hung docker
	// call holds the single slot above forever, and capacity() inside the
	// GetMessage loop, the sweep ticker and provisioning all stall behind it with
	// nothing left running to notice.
	engineCallTimeout = 2 * time.Minute
	// jitStartTimeout is far longer because the first jit-start on a fresh host
	// pulls the runner image before the container can start. Two minutes would
	// kill a legitimately slow pull and report it as an engine failure.
	jitStartTimeout = 30 * time.Minute
)

func engineTimeout(verb string) time.Duration {
	if verb == "jit-start" {
		return jitStartTimeout
	}
	return engineCallTimeout
}

// classifyExit maps the engine's exit code. Contract section 2: 0, 75, 1 and
// nothing else.
func classifyExit(code int) error {
	switch code {
	case 0:
		return nil
	case exitBusy:
		return errEngineBusy
	default:
		return errEngineFatal
	}
}

// output runs one engine verb and returns stdout. stderr is drained into the
// daemon log instead of the return value: err() writes there and the log IS the
// payload the UI tails, but a stray docker warning mixed into stdout would
// corrupt a JSON body (the same reason the PHP side has run() and run_json()).
func (e *Engine) output(ctx context.Context, args ...string) ([]byte, error) {
	verb := args[0]
	timeout := engineTimeout(verb)

	// The deadline covers the queue wait as well as the subprocess: a caller that
	// has been waiting two minutes for the slot is already past the point where
	// its answer is useful, and letting it queue is how a single stuck call turned
	// into an unbounded pile-up.
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case e.sem <- struct{}{}:
	case <-callCtx.Done():
		return nil, fmt.Errorf("%s: %w (waiting for the engine queue: %v)", verb, errEngineBusy, callCtx.Err())
	}
	defer func() { <-e.sem }()

	argv := append([]string{"--fleet", e.fleet}, args...)
	cmd := exec.CommandContext(callCtx, e.path, argv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	e.logStderr(verb, stderr.String())

	if err != nil {
		// A call killed by our own deadline is back-pressure, not a fault. It
		// surfaces as exit code -1, which classifyExit folds into fatal, and that
		// would latch the whole fleet unhealthy over one slow docker.
		if ctx.Err() == nil && callCtx.Err() != nil {
			return nil, fmt.Errorf("%s: %w (timed out after %s)", verb, errEngineBusy, timeout)
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// ExitCode is -1 when the process was signalled, which classifyExit
			// folds into fatal rather than guessing it was transient.
			return nil, fmt.Errorf("%s: %w (exit %d)", verb, classifyExit(ee.ExitCode()), ee.ExitCode())
		}
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	return stdout.Bytes(), nil
}

func (e *Engine) logStderr(verb, s string) {
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if line != "" {
			e.log.Warn("engine", "verb", verb, "msg", line)
		}
	}
}

// Runner is one row of `jit-list`: a container the engine manages, joined to the
// GitHub-side runner name via the container label. The engine owns ci-runner-N,
// GitHub owns the minted name, and this label is the only join between them.
type Runner struct {
	Container    string `json:"container"`
	GHRunnerName string `json:"gh_runner_name"`
	Index        int    `json:"index"`
	Running      bool   `json:"running"`
}

type runnerList struct {
	Runners []Runner `json:"runners"`
}

func (e *Engine) JitList(ctx context.Context) ([]Runner, error) {
	out, err := e.output(ctx, "jit-list")
	if err != nil {
		return nil, err
	}
	return parseRunnerList(out)
}

func parseRunnerList(b []byte) ([]Runner, error) {
	var list runnerList
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("parsing jit-list: %w", err)
	}
	return list.Runners, nil
}

func (e *Engine) JitStart(ctx context.Context, blobPath, ghRunnerName string) error {
	_, err := e.output(ctx, "jit-start", blobPath, ghRunnerName)
	return err
}

func (e *Engine) JitRemove(ctx context.Context, container string) error {
	_, err := e.output(ctx, "jit-remove", container)
	return err
}

func (e *Engine) Sweep(ctx context.Context) error {
	_, err := e.output(ctx, "sweep")
	return err
}

// retryBusy retries only errEngineBusy. A fatal error is returned immediately so
// the caller can mark the fleet unhealthy: retrying a cache-root guard failure
// or an unreachable Docker just hides it behind a slow loop.
func retryBusy(ctx context.Context, log *slog.Logger, what string, fn func() error) error {
	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		err := fn()
		if !errors.Is(err, errEngineBusy) {
			return err
		}
		if attempt >= busyMaxAttempts {
			return err
		}
		log.Info("engine reported back-pressure, retrying", "what", what, "attempt", attempt, "in", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = nextBackoff(backoff, busyMaxBackoff)
	}
}

const (
	busyMaxAttempts = 5
	busyMaxBackoff  = 30 * time.Second
)

// nextBackoff doubles up to a ceiling. Deliberately without jitter: there is one
// daemon per fleet on one host, so there is no thundering herd to spread out.
func nextBackoff(cur, max time.Duration) time.Duration {
	next := cur * 2
	if next > max {
		return max
	}
	return next
}
