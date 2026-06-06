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
	"github.com/konih/kollect/internal/spoke"
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

		result := ctrl.Result{RequeueAfter: r.exportDebounce(inv)}
		if kollecterrors.IsTerminal(err) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	return removeFinalizerAndUpdate(ctx, r.Client, inv, inventoryCleanupFinalizer)
}

func (r *KollectInventoryReconciler) cleanupInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	if err := spoke.PublishInventoryDeletion(ctx, r.Store, inv); err != nil {
		return err
	}

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
