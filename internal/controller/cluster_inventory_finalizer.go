// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/metrics"
	"github.com/platformrelay/kollect/internal/sink"
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

	// K-30 escape hatch (see the namespaced finalizer for semantics).
	if forceCleanupRequested(inv.Annotations) {
		recordCleanupForced(r.Recorder, inv, clusterInventoryCleanupFinalizer)

		return removeFinalizerAndUpdate(ctx, r.Client, inv, clusterInventoryCleanupFinalizer)
	}

	report, err := r.cleanupClusterInventorySinks(ctx, inv)
	recordCleanupAnnouncements(r.Recorder, inv, report)

	if err != nil {
		logf.FromContext(ctx).Error(err, "cluster inventory sink cleanup failed", "inventory", inv.Name)

		if kollecterrors.IsTerminal(err) {
			// Returning a non-nil error would make controller-runtime requeue
			// with backoff, defeating the no-requeue intent for terminal errors.
			msg := fmt.Sprintf(
				"sink cleanup failed terminally: %v — fix the sink configuration, set the %q annotation to \"true\", or remove the %q finalizer manually",
				err, kollectdevv1alpha1.AnnotationForceCleanup, clusterInventoryCleanupFinalizer)
			recordWarning(r.Recorder, inv, reasonCleanupTerminal, msg)
			// K-30: the wedge needs a counter an operator can alert on.
			metrics.CleanupTerminalTotal.WithLabelValues("cluster-inventory").Inc()
			// Best-effort Degraded status: the object is deleting, update errors are ignored.
			_, _ = r.setDegraded(ctx, inv, reasonCleanupTerminal, msg)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, err
	}

	return removeFinalizerAndUpdate(ctx, r.Client, inv, clusterInventoryCleanupFinalizer)
}

func (r *KollectClusterInventoryReconciler) cleanupClusterInventorySinks(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) (SinkCleanupReport, error) {
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
