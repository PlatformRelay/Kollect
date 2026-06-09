// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
)

const inventoryCleanupFinalizer = "kollect.dev/inventory-cleanup"

func (r *KollectInventoryReconciler) ensureInventoryFinalizer(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	return ensureFinalizer(ctx, r.Client, inv, inventoryCleanupFinalizer)
}

func (r *KollectInventoryReconciler) finalizeInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (ctrl.Result, error) {
	if !containsFinalizer(inv.Finalizers, inventoryCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupInventoryDeletion(ctx, inv); err != nil {
		logf.FromContext(ctx).Error(err, "inventory cleanup failed",
			"inventory", inv.Name, "namespace", inv.Namespace)

		if kollecterrors.IsTerminal(err) {
			// Returning a non-nil error would make controller-runtime requeue
			// with backoff, defeating the no-requeue intent for terminal errors.
			msg := fmt.Sprintf(
				"sink cleanup failed terminally: %v — fix the sink configuration or remove the %q finalizer manually",
				err, inventoryCleanupFinalizer)
			recordWarning(r.Recorder, inv, reasonCleanupTerminal, msg)
			// Best-effort Degraded status: the object is deleting, update errors are ignored.
			_, _ = r.setInventoryDegraded(ctx, inv, inv.Status.ItemCount, reasonCleanupTerminal, msg)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, err
	}

	return removeFinalizerAndUpdate(ctx, r.Client, inv, inventoryCleanupFinalizer)
}

func (r *KollectInventoryReconciler) cleanupInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	return cleanupSinkExports(
		ctx,
		r.Client,
		r.Registry,
		inv.Namespace,
		inventorySinkBindings(inv),
		false,
		fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name),
		inv.Generation,
	)
}
