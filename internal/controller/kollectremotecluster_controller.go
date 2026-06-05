// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/remotecredentials"
)

const remoteClusterRequeueInterval = 2 * time.Minute

// KollectRemoteClusterReconciler maintains minimal Connected status for registered spokes (ADR-0503).
type KollectRemoteClusterReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options RuntimeOptions

	// APIChecker probes spoke API health from credentialsSecretRef (nil uses DefaultAPIChecker).
	APIChecker remotecredentials.APIChecker
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

	r.reconcileConnected(&rc)
	r.reconcileCredentials(ctx, &rc)

	if err := r.Status().Update(ctx, &rc); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		metrics.ReconcileErrorsTotal.WithLabelValues("KollectRemoteCluster", metrics.ErrorClassTransient).Inc()
		log.Error(err, "update remote cluster status")
		retErr = err

		return ctrl.Result{}, err
	}

	if cond := apimeta.FindStatusCondition(rc.Status.Conditions, kollectdevv1alpha1.ConditionConnected); cond != nil &&
		cond.Status == metav1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: remoteClusterRequeueInterval}, nil
}

func (r *KollectRemoteClusterReconciler) reconcileConnected(rc *kollectdevv1alpha1.KollectRemoteCluster) {
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
}

func (r *KollectRemoteClusterReconciler) reconcileCredentials(
	ctx context.Context,
	rc *kollectdevv1alpha1.KollectRemoteCluster,
) {
	if rc.Spec.CredentialsSecretRef == nil || strings.TrimSpace(rc.Spec.CredentialsSecretRef.Name) == "" {
		apimeta.RemoveStatusCondition(&rc.Status.Conditions, kollectdevv1alpha1.ConditionCredentialsVerified)

		return
	}

	secretName := strings.TrimSpace(rc.Spec.CredentialsSecretRef.Name)
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: rc.Namespace, Name: secretName}

	if err := r.Get(ctx, key, &secret); err != nil {
		reason := "SecretLoadFailed"
		class := metrics.ErrorClassTransient
		if apierrors.IsNotFound(err) {
			reason = "SecretNotFound"
			class = metrics.ErrorClassTerminal
		}

		metrics.ReconcileErrorsTotal.WithLabelValues("KollectRemoteCluster", class).Inc()
		apimeta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionCredentialsVerified,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            fmt.Sprintf("credentials secret %q: %v", secretName, err),
			ObservedGeneration: rc.Generation,
			LastTransitionTime: metav1.Now(),
		})

		return
	}

	checker := r.APIChecker
	if checker == nil {
		checker = remotecredentials.DefaultAPIChecker{}
	}

	if err := remotecredentials.VerifySecret(ctx, &secret, rc.Spec.ClusterName, checker); err != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectRemoteCluster", metrics.ErrorClassTransient).Inc()
		log := logf.FromContext(ctx)
		log.Info("remote credentials verification failed", "secret", secretName, "error", err)
		apimeta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionCredentialsVerified,
			Status:             metav1.ConditionFalse,
			Reason:             "CredentialsInvalid",
			Message:            err.Error(),
			ObservedGeneration: rc.Generation,
			LastTransitionTime: metav1.Now(),
		})

		return
	}

	apimeta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionCredentialsVerified,
		Status:             metav1.ConditionTrue,
		Reason:             "CredentialsVerified",
		Message:            fmt.Sprintf("kubeconfig in secret %q verified for cluster %q", secretName, rc.Spec.ClusterName),
		ObservedGeneration: rc.Generation,
		LastTransitionTime: metav1.Now(),
	})
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
