// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const targetCleanupFinalizer = "kollect.dev/target-cleanup"

func (r *KollectTargetReconciler) ensureTargetFinalizer(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) error {
	return ensureFinalizer(ctx, r.Client, target, targetCleanupFinalizer)
}

func (r *KollectTargetReconciler) reconcileTargetFinalizers(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (ctrl.Result, bool, error) {
	if !target.DeletionTimestamp.IsZero() {
		result, err := r.finalizeTargetDeletion(ctx, target)

		return result, true, err
	}

	if err := r.ensureTargetFinalizer(ctx, target); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, true, nil
		}

		return ctrl.Result{}, true, err
	}

	return ctrl.Result{}, false, nil
}

func (r *KollectTargetReconciler) finalizeTargetDeletion(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (ctrl.Result, error) {
	if !containsFinalizer(target.Finalizers, targetCleanupFinalizer) {
		return ctrl.Result{}, nil
	}

	if r.Engine != nil {
		r.Engine.UnregisterTarget(target.Namespace, target.Name)
	}

	return removeFinalizerAndUpdate(ctx, r.Client, target, targetCleanupFinalizer)
}

func containsFinalizer(finalizers []string, name string) bool {
	for _, f := range finalizers {
		if f == name {
			return true
		}
	}

	return false
}
