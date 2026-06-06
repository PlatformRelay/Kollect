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
	"github.com/konih/kollect/internal/sink"
)

const clusterInventoryCleanupFinalizer = "kollect.dev/cluster-inventory-cleanup"

func (r *KollectClusterInventoryReconciler) ensureClusterInventoryFinalizer(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) error {
	return ensureFinalizer(ctx, r.Client, inv, clusterInventoryCleanupFinalizer)
}

func (r *KollectClusterInventoryReconciler) finalizeClusterInventoryDeletion(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) (ctrl.Result, error) {
	if !containsFinalizer(inv.Finalizers, clusterInventoryCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupClusterInventorySinks(ctx, inv); err != nil {
		logf.FromContext(ctx).Error(err, "cluster inventory sink cleanup failed", "inventory", inv.Name)

		result := ctrl.Result{RequeueAfter: r.exportDebounce(inv)}
		if kollecterrors.IsTerminal(err) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	return removeFinalizerAndUpdate(ctx, r.Client, inv, clusterInventoryCleanupFinalizer)
}

func (r *KollectClusterInventoryReconciler) cleanupClusterInventorySinks(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) error {
	sinkNS := inv.Spec.SinkNamespace
	if sinkNS == "" {
		sinkNS = sink.DefaultSecretNamespace
	}

	return cleanupSinkExports(
		ctx,
		r.Client,
		r.Registry,
		sinkNS,
		clusterInventorySinkBindings(inv),
		true,
		fmt.Sprintf("inventory/cluster/%s.json", inv.Name),
		inv.Generation,
	)
}
