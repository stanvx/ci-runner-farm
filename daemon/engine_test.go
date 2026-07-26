package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}

// Contract section 2: the engine speaks exactly three exit codes to the daemon.
// Getting 75 wrong is the expensive one — treating back-pressure as a hard fault
// would mark a healthy fleet unhealthy the first time the host filled up.
func TestClassifyExit(t *testing.T) {
	tests := []struct {
		code int
		want error
	}{
		{0, nil},
		{75, errEngineBusy},
		{1, errEngineFatal},
		{2, errEngineFatal},
		{127, errEngineFatal},
		{-1, errEngineFatal}, // killed by a signal
	}

	for _, tc := range tests {
		got := classifyExit(tc.code)
		if !errors.Is(got, tc.want) {
			t.Fatalf("classifyExit(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestParseRunnerList(t *testing.T) {
	const doc = `{"runners":[
	  {"container":"ci-runner-1","gh_runner_name":"unraid-abc123","index":1,"running":true},
	  {"container":"ci-runner-2","gh_runner_name":"unraid-def456","index":2,"running":false}
	]}`

	runners, err := parseRunnerList([]byte(doc))
	if err != nil {
		t.Fatalf("parseRunnerList: %v", err)
	}
	if len(runners) != 2 {
		t.Fatalf("got %d runners, want 2", len(runners))
	}
	if runners[0].GHRunnerName != "unraid-abc123" || !runners[0].Running || runners[0].Index != 1 {
		t.Fatalf("unexpected first runner: %+v", runners[0])
	}
	if runners[1].Running {
		t.Fatalf("second runner should be exited: %+v", runners[1])
	}

	// An empty fleet is the normal case at boot, not an error.
	empty, err := parseRunnerList([]byte(`{"runners":[]}`))
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty list: %v / %v", empty, err)
	}
	if _, err := parseRunnerList([]byte("not json")); err == nil {
		t.Fatal("a corrupt jit-list body must be an error, not an empty fleet")
	}
}

func TestNextBackoff(t *testing.T) {
	tests := []struct{ cur, max, want time.Duration }{
		{2 * time.Second, 30 * time.Second, 4 * time.Second},
		{16 * time.Second, 30 * time.Second, 30 * time.Second},
		{30 * time.Second, 30 * time.Second, 30 * time.Second},
	}
	for _, tc := range tests {
		if got := nextBackoff(tc.cur, tc.max); got != tc.want {
			t.Fatalf("nextBackoff(%v, %v) = %v, want %v", tc.cur, tc.max, got, tc.want)
		}
	}
}
