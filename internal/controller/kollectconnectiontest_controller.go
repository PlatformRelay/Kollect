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
	"github.com/konih/kollect/internal/sink"
)

const defaultConnectionTestTTLSeconds = 300

// KollectConnectionTestReconciler runs one-shot sink connectivity probes.
type KollectConnectionTestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectconnectiontests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectconnectiontests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *KollectConnectionTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectconnectiontest")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var test kollectdevv1alpha1.KollectConnectionTest
	if err := r.Get(ctx, req.NamespacedName, &test); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if test.Status.Completed && test.Status.ObservedGeneration == test.Generation {
		return r.reconcileTTL(ctx, &test)
	}

	var sinkObj kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, client.ObjectKey{Namespace: test.Namespace, Name: test.Spec.SinkRef}, &sinkObj); err != nil {
		retErr = err
		res, setErr := r.setProbeFailed(
			ctx, &test, reasonSinkNotFound,
			fmt.Sprintf("KollectSink %q: %v", test.Spec.SinkRef, err),
		)

		return res, setErr
	}

	if err := r.ensureOwnerReference(ctx, &test, &sinkObj); err != nil {
		log.Error(err, "set ownerReference")
		retErr = err

		return ctrl.Result{}, err
	}

	start := time.Now()
	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, sinkObj.Spec, test.Namespace)
	if err != nil {
		retErr = err
		res, setErr := r.setProbeFailed(ctx, &test, "SecretResolveFailed", err.Error())

		return res, setErr
	}

	okMessage, testErr := sink.RunConnectionTest(ctx, sinkObj.Spec, buildCtx)
	elapsed := time.Since(start)
	test.Status.LatencyMs = elapsed.Milliseconds()

	if testErr != nil {
		log.Error(testErr, "connection test failed", "sink", test.Spec.SinkRef)
		metrics.SinkConnectionTestTotal.WithLabelValues(sinkObj.Spec.Type, metrics.ResultFailure).Inc()
		retErr = testErr

		res, setErr := r.setProbeFailed(ctx, &test, "ConnectionTestFailed", testErr.Error())

		return res, setErr
	}

	metrics.SinkConnectionTestTotal.WithLabelValues(sinkObj.Spec.Type, metrics.ResultSuccess).Inc()

	return r.setProbeSucceeded(ctx, &test, okMessage)
}

func (r *KollectConnectionTestReconciler) ensureOwnerReference(
	ctx context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
	sinkObj *kollectdevv1alpha1.KollectSink,
) error {
	if test.Spec.OwnerSink != nil && !*test.Spec.OwnerSink {
		return nil
	}

	for _, ref := range test.OwnerReferences {
		if ref.UID == sinkObj.UID {
			return nil
		}
	}

	base := test.DeepCopy()
	if err := ctrl.SetControllerReference(sinkObj, test, r.Scheme); err != nil {
		return err
	}

	return r.Patch(ctx, test, client.MergeFrom(base))
}

func (r *KollectConnectionTestReconciler) setProbeSucceeded(
	ctx context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
	message string,
) (ctrl.Result, error) {
	now := metav1.Now()
	test.Status.ObservedGeneration = test.Generation
	test.Status.Completed = true
	test.Status.CompletedAt = &now
	apimeta.SetStatusCondition(&test.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionTrue,
		Reason:             "ConnectionOK",
		Message:            message,
		ObservedGeneration: test.Generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&test.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ProbeSucceeded",
		Message:            message,
		ObservedGeneration: test.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, test); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectConnectionTestReconciler) setProbeFailed(
	ctx context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
	reason, message string,
) (ctrl.Result, error) {
	now := metav1.Now()
	test.Status.ObservedGeneration = test.Generation
	test.Status.Completed = true
	test.Status.CompletedAt = &now
	apimeta.SetStatusCondition(&test.Status.Conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: test.Generation,
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(&test.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: test.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, test); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectConnectionTestReconciler) reconcileTTL(
	ctx context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
) (ctrl.Result, error) {
	if test.Status.CompletedAt == nil {
		return ctrl.Result{}, nil
	}

	ttl := connectionTestTTL(test)
	elapsed := time.Since(test.Status.CompletedAt.Time)
	if elapsed >= ttl {
		if err := r.Delete(ctx, test); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: ttl - elapsed}, nil
}

func connectionTestTTL(test *kollectdevv1alpha1.KollectConnectionTest) time.Duration {
	secs := int64(defaultConnectionTestTTLSeconds)
	if test.Spec.TTLSecondsAfterFinished != nil {
		secs = int64(*test.Spec.TTLSecondsAfterFinished)
	}

	if secs < 0 {
		secs = 0
	}

	return time.Duration(secs) * time.Second
}

// SetupWithManager registers the reconciler.
func (r *KollectConnectionTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectConnectionTest{}).
		Named("kollectconnectiontest").
		Complete(r)
}
