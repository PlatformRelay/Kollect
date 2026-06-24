// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/scope"
)

func (r *KollectClusterInventoryReconciler) enforceClusterScopePolicy(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	sinkNS string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) (ctrl.Result, error) {
	clusterBinding, loadErr := scope.LoadCluster(ctx, r.Client)
	if loadErr != nil {
		return ctrl.Result{}, loadErr
	}

	if !clusterBinding.Enforced {
		return ctrl.Result{}, nil
	}

	for _, ns := range scope.ClusterInventoryStaticRefNamespaces(&inv.Spec, sinkNS) {
		if scopeErr := scope.ValidateClusterScopeStaticRefNamespace(clusterBinding.Scope, ns); scopeErr != nil {
			recordWarning(r.Recorder, inv, reasonSinkNamespaceDenied, scopeErr.Error())
			return r.setDegraded(ctx, inv, reasonSinkNamespaceDenied, scopeErr.Error())
		}
	}

	if scopeErr := scope.ValidateClusterInventoryClusterScopeSinkRefs(clusterBinding.Scope, bindings); scopeErr != nil {
		recordWarning(r.Recorder, inv, scopeReasonSinkDenied, scopeErr.Error())
		return r.setDegraded(ctx, inv, scopeReasonSinkDenied, scopeErr.Error())
	}

	return ctrl.Result{}, nil
}

func (r *KollectClusterTargetReconciler) resolveProfileOrDegrade(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) (*kollectdevv1alpha1.KollectProfile, bool, error) {
	profile, err := resolveClusterTargetProfile(ctx, r.Client, ct.Spec.ProfileRef)
	recordStaticRefResolution("KollectClusterTarget", metrics.StaticRefTypeProfile, err)
	if err == nil {
		return profile, false, nil
	}

	r.unregisterAll(ct)
	reason := reasonProfileNotFound
	if apierrors.IsForbidden(err) {
		reason = reasonProfileForbidden
		recordWarning(r.Recorder, ct, reason, err.Error())
	}
	if degErr := r.setDegraded(ctx, ct, reason, err.Error()); degErr != nil {
		return nil, false, degErr
	}

	return nil, true, nil
}

func (r *KollectClusterTargetReconciler) loadClusterScopeBinding(
	ctx context.Context,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) (scope.ClusterBinding, bool, error) {
	clusterBinding, loadErr := scope.LoadCluster(ctx, r.Client)
	if loadErr != nil {
		return scope.ClusterBinding{}, false, loadErr
	}

	if clusterBinding.Enforced {
		if scopeErr := scope.ValidateClusterScopeStaticRefNamespace(clusterBinding.Scope, ct.Spec.ProfileRef.Namespace); scopeErr != nil {
			r.unregisterAll(ct)
			recordWarning(r.Recorder, ct, scopeReasonNSDenied, scopeErr.Error())
			if degErr := r.setDegraded(ctx, ct, scopeReasonNSDenied, scopeErr.Error()); degErr != nil {
				return clusterBinding, false, degErr
			}
			return clusterBinding, true, nil
		}
	}

	return clusterBinding, false, nil
}
