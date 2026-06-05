// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RuntimeOptions configures controller parallelism and workqueue rate limiting.
type RuntimeOptions struct {
	MaxConcurrentTarget           int
	MaxConcurrentInventory        int
	MaxConcurrentClusterTarget    int
	MaxConcurrentClusterInventory int
	MaxConcurrentHub              int
	// ReconcileRateLimitBase, when > 0, sets the base delay for the per-item exponential
	// failure rate limiter on each controller. When zero, controller-runtime defaults apply
	// (5ms base, 1000s max — see controller-runtime pkg/controller/controller.go).
	ReconcileRateLimitBase time.Duration
}

// DefaultRuntimeOptions returns production-oriented defaults (ADR-0603).
func DefaultRuntimeOptions() RuntimeOptions {
	return RuntimeOptions{
		MaxConcurrentTarget:           5,
		MaxConcurrentInventory:        3,
		MaxConcurrentClusterTarget:    2,
		MaxConcurrentClusterInventory: 2,
		MaxConcurrentHub:              2,
	}
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
