// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// conflictRequeueAfter is the fixed requeue delay after an optimistic-concurrency
// conflict on an object update. It replaces the deprecated Result.Requeue (rate
// limiter backoff): conflicts are transient and the watch event for the update
// re-enqueues the object anyway, so a short bounded floor converges faster than
// an exponential limiter without hammering a persistently contended object.
const conflictRequeueAfter = time.Second

func ensureFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	controllerutil.AddFinalizer(obj, finalizer)

	return c.Update(ctx, obj)
}

func removeFinalizerAndUpdate(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	finalizer string,
) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: conflictRequeueAfter}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
