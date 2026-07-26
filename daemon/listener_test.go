package main

import (
	"testing"
	"time"
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
