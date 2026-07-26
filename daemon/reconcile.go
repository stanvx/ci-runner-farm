package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/actions/scaleset"
)

// reconcilePlan is derived fresh from `jit-list` on every startup. The daemon
// persists NOTHING (ADR-0003): container labels are the only state, so a crashed
// or upgraded daemon rebuilds the same picture instead of trusting a file that
// may describe a host that has since rebooted.
type reconcilePlan struct {
	// Sweep is set when any container has exited. sweep is the collector; the
	// daemon is never the right remover for a finished job.
	Sweep bool
	// Stale are GitHub runner names whose container is gone or exited. Their
	// runner record outlives the container when a JIT config was minted for a
	// container that then died, and the record occupies a scale set slot forever.
	Stale []string
	// Busy are containers still running a job. Left strictly alone.
	Busy []string
}

func planReconcile(runners []Runner) reconcilePlan {
	var p reconcilePlan
	for _, r := range runners {
		if r.Running {
			p.Busy = append(p.Busy, r.Container)
			continue
		}
		p.Sweep = true
		if r.GHRunnerName != "" {
			p.Stale = append(p.Stale, r.GHRunnerName)
		}
	}
	return p
}

// reconcile runs the plan. Ordering matters: drop the GitHub-side record first,
// because sweep destroys the container label that is the only join back to the
// runner name.
//
// ponytail: this only reaches orphans that still have a container row. scaleset
// v0.4.0 exports no list-runners call (only GetRunnerByName), so a record whose
// container was removed by hand outside the plugin is invisible here and ages
// out on GitHub's own schedule. Upgrade path: a list endpoint, if one is ever
// exported.
func reconcile(ctx context.Context, eng *Engine, c *scaleset.Client, log *slog.Logger) error {
	runners, err := eng.JitList(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}
	p := planReconcile(runners)
	log.Info("reconciling", "busy", len(p.Busy), "stale", len(p.Stale), "sweep", p.Sweep)

	for _, name := range p.Stale {
		if err := removeRunnerByName(ctx, c, name); err != nil {
			// A failure here leaks one scale set slot, which is worth a loud line
			// but not worth refusing to start the listener over.
			log.Warn("could not remove stale runner record", "runner", name, "err", err)
		}
	}
	if p.Sweep {
		if err := eng.Sweep(ctx); err != nil {
			return fmt.Errorf("reconcile sweep: %w", err)
		}
	}
	return nil
}

func removeRunnerByName(ctx context.Context, c *scaleset.Client, name string) error {
	ref, err := c.GetRunnerByName(ctx, name)
	if err != nil {
		return err
	}
	if ref == nil {
		return nil // already gone; reconcile is idempotent by design
	}
	return c.RemoveRunner(ctx, int64(ref.ID))
}
