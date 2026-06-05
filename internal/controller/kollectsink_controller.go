// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
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
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/sink/git"
	kafkasink "github.com/konih/kollect/internal/sink/kafka"
	"github.com/konih/kollect/internal/sink/postgres"
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

	var sinkObj kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, req.NamespacedName, &sinkObj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !shouldTestConnection(&sinkObj) {
		return ctrl.Result{}, nil
	}

	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, sinkObj.Spec, sink.DefaultSecretNamespace)
	if err != nil {
		return r.setConnectionFailed(ctx, &sinkObj, "SecretResolveFailed", err.Error())
	}

	okMessage, testErr := r.runConnectionTest(ctx, sinkObj.Spec, buildCtx)
	if testErr != nil {
		log.Error(testErr, "connection test failed", "type", sinkObj.Spec.Type)
		metrics.SinkConnectionTestTotal.WithLabelValues(sinkObj.Spec.Type, metrics.ResultFailure).Inc()

		return r.setConnectionFailed(ctx, &sinkObj, "ConnectionTestFailed", testErr.Error())
	}

	metrics.SinkConnectionTestTotal.WithLabelValues(sinkObj.Spec.Type, metrics.ResultSuccess).Inc()

	return r.setConnectionVerified(ctx, &sinkObj, okMessage)
}

func (r *KollectSinkReconciler) runConnectionTest(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx sink.BuildContext,
) (string, error) {
	switch spec.Type {
	case "git":
		cfg, err := git.ConfigFromSpec(spec, buildCtx.CAPEM)
		if err != nil {
			return "", err
		}

		if err := git.TestConnection(ctx, cfg); err != nil {
			return "", err
		}

		return "TLS and git remote reachability verified", nil
	case "postgres":
		if err := postgres.TestConnection(ctx, spec, buildCtx.DatabaseSecretData); err != nil {
			return "", err
		}

		return "PostgreSQL ping succeeded", nil
	case "kafka":
		if err := kafkasink.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
			return "", err
		}

		return "Kafka broker metadata request succeeded", nil
	default:
		return "", fmt.Errorf("connection test not supported for sink type %q", spec.Type)
	}
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
	sinkObj *kollectdevv1alpha1.KollectSink,
	message string,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&sinkObj.Status.Conditions, conditionDegraded)
	r.setTLSInsecureCondition(sinkObj)
	apimeta.SetStatusCondition(&sinkObj.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionTrue,
		Reason:             "ConnectionOK",
		Message:            message,
		ObservedGeneration: sinkObj.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, sinkObj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectSinkReconciler) setConnectionFailed(
	ctx context.Context,
	sinkObj *kollectdevv1alpha1.KollectSink,
	reason, message string,
) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&sinkObj.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sinkObj.Generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&sinkObj.Status.Conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sinkObj.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, sinkObj); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectSinkReconciler) setTLSInsecureCondition(sinkObj *kollectdevv1alpha1.KollectSink) {
	insecure := sinkObj.Spec.TLS != nil && sinkObj.Spec.TLS.InsecureSkipVerify
	if !insecure {
		apimeta.RemoveStatusCondition(&sinkObj.Status.Conditions, kollectdevv1alpha1.ConditionTLSInsecure)

		return
	}

	apimeta.SetStatusCondition(&sinkObj.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionTLSInsecure,
		Status:             metav1.ConditionTrue,
		Reason:             "InsecureSkipVerify",
		Message:            "TLS certificate verification is disabled; use only for development",
		ObservedGeneration: sinkObj.Generation,
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
