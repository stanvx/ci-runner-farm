package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/actions/scaleset"
	"github.com/golang-jwt/jwt/v4"
)

// drain_timeout is documented as seconds, with 0 meaning "do not wait" in
// default.cfg, the settings form and the engine. Reading 0 as "wait forever"
// turned an operator asking for an immediate Stop into a shutdown that hung.
func TestDrainTimeout(t *testing.T) {
	tests := []struct {
		cfg  int
		want time.Duration
	}{
		{600, 600 * time.Second},
		{1, time.Second},
		{0, 0},
		{-5, 0},
	}
	for _, tc := range tests {
		l := &listener{cfg: &Config{DrainTimeout: tc.cfg}}
		if got := l.drainTimeout(); got != tc.want {
			t.Fatalf("drainTimeout(%d) = %v, want %v", tc.cfg, got, tc.want)
		}
	}
}

func TestCapacityAdvertisesTotalLimit(t *testing.T) {
	l := &listener{cfg: &Config{MaxRunners: 4}}
	if got := l.capacity(context.Background()); got != 4 {
		t.Fatalf("capacity = %d, want total limit 4", got)
	}
	l.unhealthy.Store(true)
	if got := l.capacity(context.Background()); got != 0 {
		t.Fatalf("unhealthy capacity = %d, want 0", got)
	}
	l.unhealthy.Store(false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := l.capacity(ctx); got != 0 {
		t.Fatalf("cancelled capacity = %d, want 0", got)
	}
}

func TestDesiredRunnerCountUsesCurrentStatistics(t *testing.T) {
	l := &listener{cfg: &Config{MaxRunners: 3, MinRunners: 1}}
	for _, tc := range []struct{ assigned, want int }{{2, 2}, {20, 3}, {0, 0}, {-1, 0}} {
		l.updateDesired(&scaleset.RunnerScaleSetStatistic{TotalAssignedJobs: tc.assigned})
		if l.desiredJobs != tc.want {
			t.Fatalf("assigned %d: desired = %d, want %d", tc.assigned, l.desiredJobs, tc.want)
		}
	}
	l.updateDesired(nil)
	if l.desiredJobs != 0 {
		t.Fatal("a missing statistics update must retain the last known demand")
	}
}

func listenerWithEngine(t *testing.T, body string) *listener {
	t.Helper()
	path := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRF_ENGINE", path)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &listener{cfg: &Config{MaxRunners: 2}, eng: NewEngine("default", log), log: log}
}

func TestRepeatedAssignmentDoesNotProvisionDuplicate(t *testing.T) {
	l := listenerWithEngine(t, `test "$3" = jit-list || exit 1
echo '{"runners":[{"container":"ci-runner-1","running":true}]}'
`)
	msg := &scaleset.RunnerScaleSetMessage{
		Statistics:          &scaleset.RunnerScaleSetStatistic{TotalAssignedJobs: 1},
		JobAssignedMessages: []*scaleset.JobAssigned{{}},
	}
	// No API client is installed: attempting to mint a duplicate runner panics.
	l.handle(context.Background(), nil, msg)
	l.handle(context.Background(), nil, msg)
	l.scale(context.Background()) // same reconciliation used after a poll timeout
}

func TestAssignedDemandReplacesLostRunnerWithoutNewAssignment(t *testing.T) {
	count := filepath.Join(t.TempDir(), "running")
	if err := os.WriteFile(count, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRF_TEST_COUNT", count)
	l := listenerWithEngine(t, `n=$(cat "$CRF_TEST_COUNT")
case "$3" in
  jit-list)
    printf '{"runners":['
    i=1
    while [ "$i" -le "$n" ]; do
      [ "$i" = 1 ] || printf ','
      printf '{"container":"runner-%s","running":true}' "$i"
      i=$((i+1))
    done
    printf ']}' ;;
  jit-start)
    test "$(cat "$4")" = jit-test || exit 1
    echo "$((n+1))" > "$CRF_TEST_COUNT"
    rm "$4" ;;
  *) exit 1 ;;
esac
`)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("fixture"))
	if err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/registration-token"):
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"token":"fixture"}`)
		case strings.HasSuffix(r.URL.Path, "/runner-registration"):
			json.NewEncoder(w).Encode(map[string]string{"url": srv.URL, "token": token})
		case strings.HasSuffix(r.URL.Path, "/generatejitconfig"):
			io.WriteString(w, `{"runner":{"id":1,"name":"test-runner"},"encodedJITConfig":"jit-test"}`)
		default:
			t.Errorf("unexpected API request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	l.client, err = scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL: srv.URL + "/org/repo", PersonalAccessToken: "fixture",
	}, scaleset.WithoutTLSVerify(), scaleset.WithRetryMax(0), scaleset.WithLogger(l.log))
	if err != nil {
		t.Fatal(err)
	}
	l.runDir = t.TempDir()
	l.scaleSet = &scaleset.RunnerScaleSet{ID: 1}
	l.updateDesired(&scaleset.RunnerScaleSetStatistic{TotalAssignedJobs: 2})
	l.scale(context.Background())
	l.scale(context.Background()) // poll timeout with unchanged demand must not duplicate
	if got, _ := os.ReadFile(count); strings.TrimSpace(string(got)) != "2" {
		t.Fatalf("running = %q, want 2", got)
	}
	if err := os.WriteFile(count, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	l.scale(context.Background()) // a lost container must recover without a fresh message
	if got, _ := os.ReadFile(count); strings.TrimSpace(string(got)) != "2" {
		t.Fatalf("recovered running = %q, want 2", got)
	}
}

func TestUnhealthyRecoveryRequiresSuccessfulValidation(t *testing.T) {
	verdict := filepath.Join(t.TempDir(), "validation-exit")
	t.Setenv("CRF_TEST_VERDICT", verdict)
	l := listenerWithEngine(t, `test "$3" = validate || exit 99
exit "$(cat "$CRF_TEST_VERDICT")"
`)
	l.unhealthy.Store(true)
	for _, tc := range []struct {
		exit      string
		unhealthy bool
	}{{"1", true}, {"75", true}, {"0", false}} {
		if err := os.WriteFile(verdict, []byte(tc.exit), 0o600); err != nil {
			t.Fatal(err)
		}
		l.recoverHealth(context.Background())
		if l.unhealthy.Load() != tc.unhealthy {
			t.Fatalf("validation exit %s: unhealthy = %v, want %v", tc.exit, l.unhealthy.Load(), tc.unhealthy)
		}
	}
}

func TestJITDirectoriesIsolateFleetSecrets(t *testing.T) {
	root := t.TempDir()
	a, err := prepareBlobDir(root, "default")
	if err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(a, "pending")
	if err := os.WriteFile(blob, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := prepareBlobDir(root, "dependency-submission")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("fleets must have distinct JIT directories")
	}
	if info, err := os.Stat(b); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("JIT directory must be private: %v / %v", info, err)
	}
	l := &listener{runDir: b}
	created, err := l.writeBlob("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(created); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("JIT secret file must be private: %v / %v", info, err)
	}
	if _, err := os.Stat(blob); err != nil {
		t.Fatalf("starting another fleet removed pending JIT blob: %v", err)
	}
	if _, err := prepareBlobDir(root, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blob); !os.IsNotExist(err) {
		t.Fatalf("fleet restart must purge its own stale blob: %v", err)
	}
	for _, bad := range []string{"", "..", "../default", "/tmp/other"} {
		if _, err := prepareBlobDir(root, bad); err == nil {
			t.Fatalf("accepted unsafe fleet %q", bad)
		}
	}
}

func TestAuthenticatedReadinessIsEphemeral(t *testing.T) {
	root := t.TempDir()
	dir, err := prepareBlobDir(root, "default")
	if err != nil {
		t.Fatal(err)
	}
	l := &listener{runDir: dir}
	path := filepath.Join(dir, "listener-ready")
	if err := l.setSessionReady(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := l.setSessionReady(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("closed session left readiness marker: %v", err)
	}
	if err := l.setSessionReady(false); err != nil {
		t.Fatal(err)
	}
	if err := l.setSessionReady(true); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareBlobDir(root, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("restart retained stale readiness marker: %v", err)
	}
}

func TestHardFailureReadinessRecoveryRequiresActiveSession(t *testing.T) {
	l := listenerWithEngine(t, `test "$3" = validate || exit 99
exit 0
`)
	l.runDir = t.TempDir()
	marker := filepath.Join(l.runDir, "listener-ready")
	if err := l.setSessionReady(true); err != nil {
		t.Fatal(err)
	}
	if err := l.setUnhealthy(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hard failure still shows ready: %v", err)
	}
	l.recoverHealth(context.Background())
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("active validated session did not recover readiness: %v", err)
	}
	if err := l.setUnhealthy(true); err != nil {
		t.Fatal(err)
	}
	if err := l.setSessionReady(false); err != nil {
		t.Fatal(err)
	}
	l.recoverHealth(context.Background())
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("closed session recovered readiness: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			l.setSessionReady(true)
			l.setSessionReady(false)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			l.setUnhealthy(true)
			l.setUnhealthy(false)
		}
	}()
	wg.Wait()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("concurrent health recovery left closed session ready: %v", err)
	}
}
