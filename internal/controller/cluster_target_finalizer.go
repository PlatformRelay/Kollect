// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const clusterTargetCleanupFinalizer = "kollect.dev/cluster-target-cleanup"

func (r *KollectClusterTargetReconciler) ensureClusterTargetFinalizer(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) error {
	return ensureFinalizer(ctx, r.Client, ct, clusterTargetCleanupFinalizer)
}

func (r *KollectClusterTargetReconciler) finalizeClusterTargetDeletion(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) (ctrl.Result, error) {
	if !containsFinalizer(ct.Finalizers, clusterTargetCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	r.unregisterAll(ct)

	return removeFinalizerAndUpdate(ctx, r.Client, ct, clusterTargetCleanupFinalizer)
}
