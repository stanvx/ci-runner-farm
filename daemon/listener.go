package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/actions/scaleset"
)

// defaultRunDir is RUNDIR: tmpfs, never flash. JIT config blobs decode to the
// runner's RSA private key and are written many times an hour, so putting them
// on the USB stick would be both a secret-at-rest problem and flash wear.
// CRF_RUNDIR overrides it for tests and for deploy.sh iteration.
const defaultRunDir = "/var/local/emhttp/ci-runner-farm"

const (
	// sweepInterval collects containers that exited between reconciles. Cheap:
	// sweep is a docker ps plus removals, and the fleet lock it takes is only
	// otherwise contested by a UI action.
	sweepInterval = 60 * time.Second
	// sessionBackoff bounds the reconnect loop. There is NO watchdog (ADR-0002),
	// so the daemon must survive a GitHub outage on its own rather than dying and
	// waiting for something to restart it.
	sessionBackoffMin = 5 * time.Second
	sessionBackoffMax = 5 * time.Minute
	// drainPoll is how often the drain phase re-checks for finished containers.
	drainPoll = 5 * time.Second
	// forceRemoveBudget caps the whole force-remove pass. Shutdown has to end:
	// a docker rm that hangs must not turn a bounded drain into an unbounded one.
	forceRemoveBudget = 2 * time.Minute
)

type listener struct {
	cfg      *Config
	eng      *Engine
	client   *scaleset.Client
	scaleSet *scaleset.RunnerScaleSet
	log      *slog.Logger
	runDir   string

	// lastMessageID survives a session rebuild on purpose: resetting it to 0 made
	// the new session replay messages this daemon had already acted on.
	lastMessageID int

	// unhealthy latches when the engine reports a hard failure. Provisioning
	// stops and capacity is advertised as zero, but the process stays up: a fleet
	// that reports degraded in the UI is far easier to diagnose than a daemon
	// that vanished.
	unhealthy atomic.Bool
}

// runListener is the `run --fleet <name>` subcommand.
func runListener(ctx context.Context, fleet string, log *slog.Logger) error {
	eng := NewEngine(fleet, log)

	cfg, err := startupRetry(ctx, log, "config-json", func() (*Config, error) {
		return loadConfig(ctx, eng)
	})
	if err != nil {
		return err
	}
	// validate is deliberately NOT retried: a fleet in the wrong mode or with an
	// empty SCALESET_NAME is the config saying stop, and retrying it would spin
	// forever waiting for a file only an operator can change.
	if err := cfg.validate(); err != nil {
		return err
	}
	log.Info("starting", "fleet", cfg.Fleet, "scale_set", cfg.ScaleSetName,
		"url", cfg.GitHubConfigURL, "credential", cfg.Credential.Name, "max_runners", cfg.MaxRunners)

	// newClient only reads and parses the credential files, so a failure here is
	// unrecoverable auth, not weather: exit and let the operator fix flash.
	client, err := newClient(cfg, log)
	if err != nil {
		return err
	}
	scaleSet, err := startupRetry(ctx, log, "scale set lookup", func() (*scaleset.RunnerScaleSet, error) {
		return ensureScaleSet(ctx, client, cfg, log)
	})
	if err != nil {
		return err
	}

	runDir := os.Getenv("CRF_RUNDIR")
	if runDir == "" {
		runDir = defaultRunDir
	}
	blobDir := filepath.Join(runDir, "jit")
	// Purge before recreating. A SIGKILL between writeBlob and the engine's rm -f,
	// or a failed os.Remove on a jit-start that never happened, leaves a 0600 file
	// that decodes to a runner's RSA private key sitting on tmpfs until reboot.
	// Safe with runners live: their bind-mount pins the inode, so dropping the
	// directory entry does not disturb a running job.
	if err := os.RemoveAll(blobDir); err != nil {
		return fmt.Errorf("clearing %s: %w", blobDir, err)
	}
	if err := os.MkdirAll(blobDir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", blobDir, err)
	}

	l := &listener{cfg: cfg, eng: eng, client: client, scaleSet: scaleSet, log: log, runDir: blobDir}

	if _, err := startupRetry(ctx, log, "reconcile", func() (struct{}, error) {
		return struct{}{}, reconcile(ctx, eng, client, log)
	}); err != nil {
		return err
	}

	go l.sweepLoop(ctx)
	l.serve(ctx)

	// ctx is already cancelled here, so the drain gets an uncancelled one: every
	// call it makes bounds itself, and a cancelled ctx would fail the force-remove
	// that ADR-0004 depends on.
	l.drain(context.WithoutCancel(ctx), l.drainTimeout())
	return nil
}

func (l *listener) drainTimeout() time.Duration {
	if l.cfg.DrainTimeout <= 0 {
		// 0 is "do not wait", the meaning default.cfg, the settings form and the
		// engine all document. Reading it as "wait forever" turned an operator
		// asking for an immediate Stop into a shutdown that never returned.
		return 0
	}
	return time.Duration(l.cfg.DrainTimeout) * time.Second
}

// startupRetry retries a startup step instead of exiting the process. Exit 75 is
// NORMAL per the engine contract (a contended fleet lock), and nothing restarts
// this daemon by design (ADR-0002), so exiting on one leaves the fleet silently
// acquiring no work until a human notices. Every attempt logs at ERROR, which is
// the signal the UI tails; only ctx cancellation ends the loop.
func startupRetry[T any](ctx context.Context, log *slog.Logger, what string, fn func() (T, error)) (T, error) {
	backoff := sessionBackoffMin
	for attempt := 1; ; attempt++ {
		v, err := fn()
		if err == nil || ctx.Err() != nil {
			return v, err
		}
		log.Error("startup step failed, retrying", "step", what, "attempt", attempt, "in", backoff, "err", err)
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = nextBackoff(backoff, sessionBackoffMax)
	}
}

// serve owns reconnection. A dropped session, an expired queue token, or a
// GitHub 5xx are all normal weather; only ctx cancellation ends the loop.
func (l *listener) serve(ctx context.Context) {
	backoff := sessionBackoffMin
	for ctx.Err() == nil {
		err := l.session(ctx)
		if ctx.Err() != nil {
			return
		}
		l.log.Error("message session ended, reconnecting", "err", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = nextBackoff(backoff, sessionBackoffMax)
	}
}

func (l *listener) session(ctx context.Context) error {
	owner, err := os.Hostname()
	if err != nil || owner == "" {
		owner = "unraid"
	}
	owner = fmt.Sprintf("%s-%s", owner, l.cfg.Fleet)

	sc, err := l.client.MessageSessionClient(ctx, l.scaleSet.ID, owner)
	if err != nil {
		return fmt.Errorf("opening message session: %w", err)
	}
	defer func() {
		// Closing the session is how the daemon stops being offered work: GitHub
		// will not assign jobs to a scale set with no live session. That is the
		// zero-capacity advertisement, and it must happen even on a cancelled ctx.
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := sc.Close(closeCtx); err != nil {
			l.log.Warn("closing message session", "err", err)
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg, err := sc.GetMessage(ctx, l.lastMessageID, l.capacity(ctx))
		if err != nil {
			// GetMessage already refreshes once on an expired queue token; if it
			// still surfaces, the whole session is rebuilt by the caller.
			if errors.Is(err, scaleset.MessageQueueTokenExpiredError) {
				return fmt.Errorf("session token could not be refreshed: %w", err)
			}
			return err
		}
		if msg == nil {
			continue // long poll expired with nothing queued
		}

		l.handle(ctx, sc, msg)

		// Advance the watermark BEFORE acknowledging, and keep it on the listener
		// so a rebuilt session resumes from it rather than from 0. GetMessage
		// treats lastMessageID as the acknowledgement, so a failed DeleteMessage
		// no longer redelivers a JobAssigned this loop already handled — which
		// used to mint a SECOND runner for a job that was already started.
		l.lastMessageID = msg.MessageID
		if err := sc.DeleteMessage(ctx, msg.MessageID); err != nil {
			l.log.Warn("could not acknowledge message", "id", msg.MessageID, "err", err)
		}
	}
}

// capacity is derived from jit-list rather than from a counter, for the same
// reason reconcile is: containers are the only state, and an in-memory count
// would drift from reality the first time a runner was removed from the UI.
func (l *listener) capacity(ctx context.Context) int {
	if l.unhealthy.Load() {
		return 0
	}
	runners, err := l.eng.JitList(ctx)
	if err != nil {
		l.log.Warn("could not read jit-list, advertising no capacity", "err", err)
		return 0
	}
	busy := 0
	for _, r := range runners {
		if r.Running {
			busy++
		}
	}
	if free := l.cfg.MaxRunners - busy; free > 0 {
		return free
	}
	return 0
}

func (l *listener) handle(ctx context.Context, sc *scaleset.MessageSessionClient, msg *scaleset.RunnerScaleSetMessage) {
	if len(msg.JobAvailableMessages) > 0 {
		l.acquire(ctx, sc, msg.JobAvailableMessages)
	}
	for _, job := range msg.JobAssignedMessages {
		l.provision(ctx, job)
	}
	for _, job := range msg.JobStartedMessages {
		l.log.Info("job started", "runner", job.RunnerName, "job", job.JobDisplayName, "repo", job.RepositoryName)
	}
	for _, job := range msg.JobCompletedMessages {
		// Accounting only. The container exits on its own and sweep collects it;
		// removing it here would race a runner still uploading its logs.
		l.log.Info("job completed", "runner", job.RunnerName, "job", job.JobDisplayName,
			"repo", job.RepositoryName, "result", job.Result)
	}
}

func (l *listener) acquire(ctx context.Context, sc *scaleset.MessageSessionClient, available []*scaleset.JobAvailable) {
	if l.unhealthy.Load() {
		l.log.Warn("fleet is unhealthy, not acquiring jobs", "offered", len(available))
		return
	}
	ids := make([]int64, 0, len(available))
	for _, job := range available {
		ids = append(ids, job.RunnerRequestID)
	}
	acquired, err := sc.AcquireJobs(ctx, ids)
	if err != nil {
		l.log.Error("acquiring jobs", "count", len(ids), "err", err)
		return
	}
	l.log.Info("acquired jobs", "offered", len(ids), "acquired", len(acquired))
}

func (l *listener) provision(ctx context.Context, job *scaleset.JobAssigned) {
	if l.unhealthy.Load() {
		l.log.Error("fleet is unhealthy, not provisioning", "job", job.JobDisplayName)
		return
	}

	name, err := runnerName(l.cfg.ScaleSetName)
	if err != nil {
		l.log.Error("generating runner name", "err", err)
		return
	}

	jit, err := l.client.GenerateJitRunnerConfig(ctx, &scaleset.RunnerScaleSetJitRunnerSetting{
		Name: name,
		// WorkFolder MUST be absolute. Left unset it defaults to _work relative
		// to the runner root, which lands the work dir on the container overlay
		// instead of the pool-backed mount the plugin bind-mounts at /_work.
		WorkFolder: "/_work",
	}, l.scaleSet.ID)
	if err != nil {
		l.log.Error("minting JIT config", "runner", name, "err", err)
		return
	}
	// Guard before the blob is written: the runner name is the container label
	// that joins back to GitHub, and dereferencing a nil Runner panicked the whole
	// daemon and orphaned a decodable private key on tmpfs. There is no ID to
	// remove either, so there is nothing to clean up but the log line.
	if jit == nil || jit.Runner == nil || jit.Runner.Name == "" {
		l.log.Error("GitHub returned a JIT config with no runner reference", "runner", name)
		return
	}

	blob, err := l.writeBlob(jit.EncodedJITConfig)
	if err != nil {
		l.log.Error("writing JIT config", "runner", name, "err", err)
		l.removeMinted(ctx, jit)
		return
	}

	err = retryBusy(ctx, l.log, "jit-start", func() error {
		return l.eng.JitStart(ctx, blob, jit.Runner.Name)
	})
	if err == nil {
		l.log.Info("started runner", "runner", jit.Runner.Name, "job", job.JobDisplayName, "repo", job.RepositoryName)
		return
	}

	// The engine unlinks the blob only on success, so an unstarted runner leaves
	// a decodable private key on tmpfs unless it is removed here.
	if rmErr := os.Remove(blob); rmErr != nil && !os.IsNotExist(rmErr) {
		l.log.Warn("removing unused JIT config", "path", blob, "err", rmErr)
	}
	l.removeMinted(ctx, jit)

	if errors.Is(err, errEngineFatal) {
		l.unhealthy.Store(true)
		l.log.Error("engine hard failure, fleet marked unhealthy and provisioning stopped", "err", err)
		return
	}
	l.log.Error("could not start runner, GitHub will re-offer the job", "runner", name, "err", err)
}

// removeMinted drops the GitHub-side record for a runner that never started.
// Left behind it counts against the scale set until GitHub ages it out.
func (l *listener) removeMinted(ctx context.Context, jit *scaleset.RunnerScaleSetJitRunnerConfig) {
	if jit == nil || jit.Runner == nil {
		return
	}
	if err := l.client.RemoveRunner(ctx, int64(jit.Runner.ID)); err != nil {
		l.log.Warn("removing unused runner record", "runner", jit.Runner.Name, "err", err)
	}
}

// writeBlob writes the encoded JIT config 0600 under RUNDIR. It is the runner's
// credential for the life of one job, and the engine bind-mounts this exact path
// read-only rather than passing it through the environment, where it would show
// up in `docker inspect` for anyone on the LAN with the webGUI open.
func (l *listener) writeBlob(encoded string) (string, error) {
	f, err := os.CreateTemp(l.runDir, "jitconfig-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if _, err := f.WriteString(encoded); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func runnerName(prefix string) (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b[:])), nil
}

func (l *listener) sweepLoop(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := l.eng.Sweep(ctx); err != nil && ctx.Err() == nil {
				l.log.Warn("periodic sweep", "err", err)
			}
		}
	}
}

// drain lets in-flight jobs finish after SIGTERM, then force-removes whatever is
// still running when drain_timeout expires (ADR-0004). A job killed mid-run is
// reported as a failure to the user; a shutdown that hangs forever is worse, so
// the timeout is a deliberate ceiling rather than a bug.
func (l *listener) drain(ctx context.Context, budget time.Duration) {
	// The budget is wall clock, not a context. Bounding the engine calls by a
	// context derived from the drain deadline meant every jit-list failed once the
	// budget was spent, so the loop returned and the force-remove below was
	// unreachable for any drain_timeout the deadline outlived. Each engine call
	// bounds itself (engineCallTimeout), which is the only cap they need.
	deadline := time.Now().Add(budget)
	var busy []Runner

	for {
		runners, err := l.eng.JitList(ctx)
		if err != nil {
			// Without the container list there is nothing to force-remove, so this
			// exits with runners possibly still up. Loud, not silent.
			l.log.Error("drain could not read jit-list, leaving runners in place", "err", err)
			return
		}
		busy = busy[:0]
		for _, r := range runners {
			if r.Running {
				busy = append(busy, r)
			}
		}
		if len(busy) == 0 {
			l.log.Info("drain complete, no runners left")
			return
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		l.log.Info("draining", "running", len(busy), "remaining", remaining.Truncate(time.Second))
		time.Sleep(min(drainPoll, remaining))
	}

	l.forceRemove(ctx, busy)
}

// forceRemove runs after the drain budget is spent, so it derives a fresh short
// context with WithoutCancel: reusing an expired one would fail every jit-remove
// instantly and exit with the runners still running, the opposite of ADR-0004.
func (l *listener) forceRemove(ctx context.Context, busy []Runner) {
	l.log.Warn("drain timeout expired, force-removing runners", "count", len(busy))
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), forceRemoveBudget)
	defer cancel()

	for _, r := range busy {
		if err := l.eng.JitRemove(rmCtx, r.Container); err != nil {
			l.log.Error("force-removing runner", "container", r.Container, "err", err)
		}
		// jit-remove deliberately skips deregistration, because a JIT runner has
		// no long-lived registration token to hand back. Startup reconcile drops
		// the record via the same label, but nothing runs after this drain, so
		// without it every force-removed runner leaks a scale set slot.
		if r.GHRunnerName == "" {
			continue
		}
		if err := removeRunnerByName(rmCtx, l.client, r.GHRunnerName); err != nil {
			l.log.Warn("could not remove record for force-removed runner", "runner", r.GHRunnerName, "err", err)
		}
	}
}
