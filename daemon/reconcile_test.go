package main

import (
	"slices"
	"testing"
)

// The plan is derived fresh from jit-list every time. A running container must
// never appear in Stale: it holds a job, and removing its GitHub record would
// orphan a build that is still uploading logs.
func TestPlanReconcile(t *testing.T) {
	tests := []struct {
		name      string
		runners   []Runner
		wantSweep bool
		wantStale []string
		wantBusy  []string
	}{
		{
			name:    "empty fleet at boot",
			runners: nil,
		},
		{
			name:     "all busy",
			runners:  []Runner{{Container: "ci-runner-1", GHRunnerName: "unraid-a", Running: true}},
			wantBusy: []string{"ci-runner-1"},
		},
		{
			name: "exited container leaves a stale runner record",
			runners: []Runner{
				{Container: "ci-runner-1", GHRunnerName: "unraid-a", Running: true},
				{Container: "ci-runner-2", GHRunnerName: "unraid-b", Running: false},
			},
			wantSweep: true,
			wantStale: []string{"unraid-b"},
			wantBusy:  []string{"ci-runner-1"},
		},
		{
			name: "exited container that never got a label still sweeps",
			runners: []Runner{
				{Container: "ci-runner-3", GHRunnerName: "", Running: false},
			},
			wantSweep: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := planReconcile(tc.runners)
			if got.Sweep != tc.wantSweep {
				t.Fatalf("Sweep = %v, want %v", got.Sweep, tc.wantSweep)
			}
			if !slices.Equal(got.Stale, tc.wantStale) {
				t.Fatalf("Stale = %v, want %v", got.Stale, tc.wantStale)
			}
			if !slices.Equal(got.Busy, tc.wantBusy) {
				t.Fatalf("Busy = %v, want %v", got.Busy, tc.wantBusy)
			}
		})
	}
}
