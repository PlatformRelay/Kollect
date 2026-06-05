// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

const remoteClusterRequeueInterval = 2 * time.Minute

// KollectRemoteClusterReconciler maintains minimal Connected status for registered spokes (ADR-0028).
type KollectRemoteClusterReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options RuntimeOptions
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectremoteclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectremoteclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectremoteclusters/finalizers,verbs=update

// Reconcile updates ObservedGeneration and initializes Connected until hub ingest marks a report.
func (r *KollectRemoteClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectremotecluster")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var rc kollectdevv1alpha1.KollectRemoteCluster
	if err := r.Get(ctx, req.NamespacedName, &rc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	rc.Status.ObservedGeneration = rc.Generation

	connected := apimeta.FindStatusCondition(rc.Status.Conditions, kollectdevv1alpha1.ConditionConnected)
	if connected == nil || connected.ObservedGeneration != rc.Generation {
		apimeta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionConnected,
			Status:             metav1.ConditionFalse,
			Reason:             "AwaitingFirstReport",
			Message:            fmt.Sprintf("no spoke report yet for cluster %q", rc.Spec.ClusterName),
			ObservedGeneration: rc.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}

	if err := r.Status().Update(ctx, &rc); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		metrics.ReconcileErrorsTotal.WithLabelValues("KollectRemoteCluster", metrics.ErrorClassTransient).Inc()
		log.Error(err, "update remote cluster status")

		return ctrl.Result{}, err
	}

	if cond := apimeta.FindStatusCondition(rc.Status.Conditions, kollectdevv1alpha1.ConditionConnected); cond != nil &&
		cond.Status == metav1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: remoteClusterRequeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectRemoteClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentHub)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentHub
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectRemoteCluster{}).
		WithOptions(opts).
		Named("kollectremotecluster").
		Complete(r)
}
