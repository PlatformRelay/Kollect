// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink/git"
)

// KollectSinkReconciler runs connection tests and updates sink status.
type KollectSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *KollectSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sink kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, req.NamespacedName, &sink); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !shouldTestConnection(&sink) {
		return ctrl.Result{}, nil
	}

	if sink.Spec.Type != "git" {
		log.Info("connection test skipped for non-git sink", "type", sink.Spec.Type)

		return ctrl.Result{}, nil
	}

	caPEM, err := resolveCAPEM(ctx, r.Client, sink.Spec.TLS)
	if err != nil {
		return r.setConnectionFailed(ctx, &sink, "CAResolveFailed", err.Error())
	}

	cfg, err := git.ConfigFromSpec(sink.Spec, caPEM)
	if err != nil {
		return r.setConnectionFailed(ctx, &sink, "InvalidSinkConfig", err.Error())
	}

	if err := git.TestConnection(ctx, cfg); err != nil {
		metrics.SinkConnectionTestTotal.WithLabelValues(sink.Spec.Type, metrics.ResultFailure).Inc()

		return r.setConnectionFailed(ctx, &sink, "ConnectionTestFailed", err.Error())
	}

	metrics.SinkConnectionTestTotal.WithLabelValues(sink.Spec.Type, metrics.ResultSuccess).Inc()

	return r.setConnectionVerified(ctx, &sink)
}

func shouldTestConnection(sink *kollectdevv1alpha1.KollectSink) bool {
	if sink.Spec.ConnectionTest {
		return true
	}

	v, ok := sink.Annotations[kollectdevv1alpha1.AnnotationTestConnection]
	return ok && strings.EqualFold(v, "true")
}

func (r *KollectSinkReconciler) setConnectionVerified(
	ctx context.Context,
	sink *kollectdevv1alpha1.KollectSink,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&sink.Status.Conditions, conditionDegraded)
	r.setTLSInsecureCondition(sink)
	apimeta.SetStatusCondition(&sink.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionTrue,
		Reason:             "ConnectionOK",
		Message:            "TLS and git remote reachability verified",
		ObservedGeneration: sink.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, sink); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectSinkReconciler) setConnectionFailed(
	ctx context.Context,
	sink *kollectdevv1alpha1.KollectSink,
	reason, message string,
) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&sink.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sink.Generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&sink.Status.Conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sink.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, sink); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectSinkReconciler) setTLSInsecureCondition(sink *kollectdevv1alpha1.KollectSink) {
	insecure := sink.Spec.TLS != nil && sink.Spec.TLS.InsecureSkipVerify
	if !insecure {
		apimeta.RemoveStatusCondition(&sink.Status.Conditions, kollectdevv1alpha1.ConditionTLSInsecure)

		return
	}

	apimeta.SetStatusCondition(&sink.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionTLSInsecure,
		Status:             metav1.ConditionTrue,
		Reason:             "InsecureSkipVerify",
		Message:            "TLS certificate verification is disabled; use only for development",
		ObservedGeneration: sink.Generation,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager registers the reconciler.
func (r *KollectSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectSink{}).
		Named("kollectsink").
		Complete(r)
}
