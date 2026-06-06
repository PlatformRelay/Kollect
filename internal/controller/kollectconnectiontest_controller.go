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
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks;kollectdatabasesinks;kollecteventsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustersnapshotsinks;kollectclusterdatabasesinks;kollectclustereventsinks,verbs=get;list;watch
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

	family, sinkName, ok := test.Spec.SinkRef.Family()
	if !ok {
		retErr = fmt.Errorf("invalid sinkRef")
		res, setErr := r.setProbeFailed(ctx, &test, "InvalidSinkRef", "exactly one family sink ref must be set")

		return res, setErr
	}

	binding := kollectdevv1alpha1.InventorySinkBinding{Name: sinkName, Family: family}
	resolved, err := loadResolvedSink(ctx, r.Client, test.Namespace, binding, false)
	if err != nil && apierrors.IsNotFound(err) {
		resolved, err = loadResolvedSink(ctx, r.Client, test.Namespace, binding, true)
	}
	if err != nil {
		retErr = err
		res, setErr := r.setProbeFailed(
			ctx, &test, reasonSinkNotFound,
			fmt.Sprintf("%s %q: %v", familySinkKind(family, resolved != nil && resolved.ClusterScoped), sinkName, err),
		)

		return res, setErr
	}

	sinkObj, err := r.familySinkObject(ctx, resolved)
	if err != nil {
		retErr = err
		res, setErr := r.setProbeFailed(ctx, &test, reasonSinkNotFound, err.Error())

		return res, setErr
	}

	if ownRefErr := r.ensureOwnerReference(ctx, &test, sinkObj); ownRefErr != nil {
		log.Error(ownRefErr, "set ownerReference")
		retErr = ownRefErr

		return ctrl.Result{}, ownRefErr
	}

	spec := resolved.Spec
	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, spec, sink.SinkNamespaceForResolved(resolved, test.Namespace))
	if err != nil {
		retErr = err
		res, setErr := r.setProbeFailed(ctx, &test, "SecretResolveFailed", err.Error())

		return res, setErr
	}

	start := time.Now()
	okMessage, testErr := sink.RunConnectionTest(ctx, spec, buildCtx)
	elapsed := time.Since(start)
	test.Status.LatencyMs = elapsed.Milliseconds()

	if testErr != nil {
		log.Error(testErr, "connection test failed", "sink", sinkName, "family", family)
		metrics.SinkConnectionTestTotal.WithLabelValues(spec.Type, metrics.ResultFailure).Inc()
		retErr = testErr

		res, setErr := r.setProbeFailed(ctx, &test, "ConnectionTestFailed", testErr.Error())

		return res, setErr
	}

	metrics.SinkConnectionTestTotal.WithLabelValues(spec.Type, metrics.ResultSuccess).Inc()

	return r.setProbeSucceeded(ctx, &test, okMessage)
}

func (r *KollectConnectionTestReconciler) familySinkObject(
	ctx context.Context,
	resolved *sink.ResolvedSink,
) (client.Object, error) {
	if resolved == nil {
		return nil, fmt.Errorf("resolved sink is nil")
	}
	switch resolved.Family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		if resolved.ClusterScoped {
			var obj kollectdevv1alpha1.KollectClusterSnapshotSink
			if err := r.Get(ctx, client.ObjectKey{Name: resolved.Name}, &obj); err != nil {
				return nil, err
			}
			return &obj, nil
		}
		var obj kollectdevv1alpha1.KollectSnapshotSink
		if err := r.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return &obj, nil
	case kollectdevv1alpha1.SinkFamilyDatabase:
		if resolved.ClusterScoped {
			var obj kollectdevv1alpha1.KollectClusterDatabaseSink
			if err := r.Get(ctx, client.ObjectKey{Name: resolved.Name}, &obj); err != nil {
				return nil, err
			}
			return &obj, nil
		}
		var obj kollectdevv1alpha1.KollectDatabaseSink
		if err := r.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return &obj, nil
	case kollectdevv1alpha1.SinkFamilyEvent:
		if resolved.ClusterScoped {
			var obj kollectdevv1alpha1.KollectClusterEventSink
			if err := r.Get(ctx, client.ObjectKey{Name: resolved.Name}, &obj); err != nil {
				return nil, err
			}
			return &obj, nil
		}
		var obj kollectdevv1alpha1.KollectEventSink
		if err := r.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return &obj, nil
	default:
		return nil, fmt.Errorf("unknown sink family %q", resolved.Family)
	}
}

func (r *KollectConnectionTestReconciler) ensureOwnerReference(
	ctx context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
	sinkObj client.Object,
) error {
	if test.Spec.OwnerSink != nil && !*test.Spec.OwnerSink {
		return nil
	}

	for _, ref := range test.OwnerReferences {
		if ref.UID == sinkObj.GetUID() {
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
