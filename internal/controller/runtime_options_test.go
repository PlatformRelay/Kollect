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
}

func TestRuntimeOptionsControllerOptionsRateLimiter(t *testing.T) {
	t.Parallel()

	opts := RuntimeOptions{ReconcileRateLimitBase: 100 * time.Millisecond}
	got := opts.controllerOptions(3)
	if got.MaxConcurrentReconciles != 3 || got.RateLimiter == nil {
		t.Fatalf("controller options = %#v", got)
	}
}
