// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DefaultTargetCountResync bounds how stale KollectTarget.status.collectedCount may
// get. Nothing re-enqueues a target when a watched object enters or leaves its matched
// set, so without a periodic requeue the count is frozen at the last spec change
// (PERF-FIX-05 / F-05). A re-registration with unchanged state is free by design
// (collect.Engine.RegisterTarget skips the backfill on an identical fingerprint), and
// the status write is skipped unless the number actually moved, so the steady-state
// cost of the resync is one cached read per target per interval.
const DefaultTargetCountResync = 60 * time.Second

// RuntimeOptions configures controller parallelism and workqueue rate limiting.
type RuntimeOptions struct {
	MaxConcurrentTarget           int
	MaxConcurrentInventory        int
	MaxConcurrentClusterTarget    int
	MaxConcurrentClusterInventory int
	// ReconcileRateLimitBase, when > 0, sets the base delay for the per-item exponential
	// failure rate limiter on each controller. When zero, controller-runtime defaults apply
	// (5ms base, 1000s max — see controller-runtime pkg/controller/controller.go).
	ReconcileRateLimitBase time.Duration
	// TargetCountResync is how often a Ready KollectTarget is requeued to refresh
	// status.collectedCount. Zero or negative selects DefaultTargetCountResync.
	TargetCountResync time.Duration
}

// DefaultRuntimeOptions returns production-oriented defaults (ADR-0603).
func DefaultRuntimeOptions() RuntimeOptions {
	return RuntimeOptions{
		MaxConcurrentTarget:           5,
		MaxConcurrentInventory:        3,
		MaxConcurrentClusterTarget:    2,
		MaxConcurrentClusterInventory: 2,
		TargetCountResync:             DefaultTargetCountResync,
	}
}

// targetCountResync returns the configured count resync interval, falling back to the
// default so a zero-value RuntimeOptions still keeps the count live.
func (o RuntimeOptions) targetCountResync() time.Duration {
	if o.TargetCountResync > 0 {
		return o.TargetCountResync
	}

	return DefaultTargetCountResync
}

func (o RuntimeOptions) controllerOptions(maxConcurrent int) controller.Options {
	opts := controller.Options{
		MaxConcurrentReconciles: maxConcurrent,
	}
	if o.ReconcileRateLimitBase > 0 {
		maxDelay := o.ReconcileRateLimitBase * 300
		if maxDelay < o.ReconcileRateLimitBase {
			maxDelay = o.ReconcileRateLimitBase
		}

		opts.RateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
			o.ReconcileRateLimitBase,
			maxDelay,
		)
	}

	return opts
}
