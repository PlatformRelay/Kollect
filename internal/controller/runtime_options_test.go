// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"
	"time"
)

func TestDefaultRuntimeOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultRuntimeOptions()
	if opts.MaxConcurrentTarget != 5 {
		t.Fatalf("defaults = %#v", opts)
	}
	if opts.TargetCountResync != DefaultTargetCountResync {
		t.Fatalf("TargetCountResync = %s, want %s", opts.TargetCountResync, DefaultTargetCountResync)
	}
}

// PERF-FIX-05: a zero-value RuntimeOptions must still requeue, otherwise any caller that
// forgets the field silently reintroduces the frozen-count defect.
func TestRuntimeOptionsTargetCountResync(t *testing.T) {
	t.Parallel()

	if got := (RuntimeOptions{}).targetCountResync(); got != DefaultTargetCountResync {
		t.Fatalf("zero-value resync = %s, want %s", got, DefaultTargetCountResync)
	}
	if got := (RuntimeOptions{TargetCountResync: -1}).targetCountResync(); got != DefaultTargetCountResync {
		t.Fatalf("negative resync = %s, want %s", got, DefaultTargetCountResync)
	}
	if got := (RuntimeOptions{TargetCountResync: 5 * time.Second}).targetCountResync(); got != 5*time.Second {
		t.Fatalf("explicit resync = %s, want 5s", got)
	}
}

func TestRuntimeOptionsControllerOptionsRateLimiter(t *testing.T) {
	t.Parallel()

	opts := RuntimeOptions{ReconcileRateLimitBase: 100 * time.Millisecond}
	got := opts.controllerOptions(3)
	if got.MaxConcurrentReconciles != 3 || got.RateLimiter == nil {
		t.Fatalf("controller options = %#v", got)
	}
}
