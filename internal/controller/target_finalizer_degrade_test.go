// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestRemoveFinalizerAndUpdate_Conflict_RequeuesKeepingFinalizer is the finalizer EDGE lock
// (finalizer.go:32-35): an optimistic-lock Conflict while dropping the finalizer must requeue
// (Requeue=true, nil error) and the finalizer must remain on the persisted object — never let
// a conflict drop the finalizer and orphan the cleanup.
func TestRemoveFinalizerAndUpdate_Conflict_RequeuesKeepingFinalizer(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	now := metav1.Now()
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-target",
			Namespace:         "team-a",
			Finalizers:        []string{targetCleanupFinalizer},
			DeletionTimestamp: &now,
		},
	}

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollecttargets"}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(
				_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption,
			) error {
				return apierrors.NewConflict(gr, obj.GetName(), errors.New("resourceVersion conflict"))
			},
		}).
		Build()

	res, err := removeFinalizerAndUpdate(context.Background(), cl, target.DeepCopy(), targetCleanupFinalizer)
	if err != nil {
		t.Fatalf("removeFinalizerAndUpdate() error = %v, want nil (conflict is a requeue, not an error)", err)
	}
	if !res.Requeue { //nolint:staticcheck // SA1019: asserting the reconciler requeues on conflict
		t.Fatal("removeFinalizerAndUpdate() Requeue = false on conflict; want a requeue so the finalizer is retried")
	}

	var got kollectdevv1alpha1.KollectTarget
	if getErr := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got); getErr != nil {
		t.Fatalf("Get target: %v", getErr)
	}
	if !containsFinalizer(got.Finalizers, targetCleanupFinalizer) {
		t.Fatal("persisted finalizer was dropped after a conflict; the cleanup finalizer must survive to be retried")
	}
}

// TestRemoveFinalizerAndUpdate_NonConflictError_Propagates covers the terminal-error branch
// (finalizer.go:36-37): a non-conflict Update failure propagates as an error.
func TestRemoveFinalizerAndUpdate_NonConflictError_Propagates(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "err-target",
			Namespace:  "team-a",
			Finalizers: []string{targetCleanupFinalizer},
		},
	}

	sentinel := errors.New("boom")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(
				_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption,
			) error {
				return sentinel
			},
		}).
		Build()

	if _, err := removeFinalizerAndUpdate(context.Background(), cl, target.DeepCopy(), targetCleanupFinalizer); !errors.Is(err, sentinel) {
		t.Fatalf("removeFinalizerAndUpdate() error = %v, want the non-conflict error to propagate", err)
	}
}

// TestFinalizeTargetDeletion_RemovesFinalizer covers the happy deletion path
// (target_finalizer.go:45-57): a target under deletion with the finalizer present has it
// removed and the engine registration torn down.
func TestFinalizeTargetDeletion_RemovesFinalizer(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	now := metav1.Now()
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cleanup-target",
			Namespace:         "team-a",
			Finalizers:        []string{targetCleanupFinalizer},
			DeletionTimestamp: &now,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme}

	res, done, err := r.reconcileTargetFinalizers(context.Background(), target)
	if err != nil {
		t.Fatalf("reconcileTargetFinalizers() error = %v", err)
	}
	if !done {
		t.Fatal("reconcileTargetFinalizers() done = false on a deleting object; want the finalizer branch to short-circuit reconcile")
	}
	if res.Requeue { //nolint:staticcheck // SA1019: asserting no requeue on clean removal
		t.Fatalf("unexpected requeue on clean finalizer removal: %+v", res)
	}

	// The object is deleted by the fake client once its last finalizer is removed.
	var got kollectdevv1alpha1.KollectTarget
	getErr := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got)
	if getErr == nil && containsFinalizer(got.Finalizers, targetCleanupFinalizer) {
		t.Fatal("finalizer still present after clean removal")
	}
}

// TestReconcileTargetFinalizers_EnsureConflict_Requeues covers the add-finalizer conflict
// branch (target_finalizer.go:34-36): a conflict while adding the cleanup finalizer requeues
// (done=true, nil error) rather than erroring loud.
func TestReconcileTargetFinalizers_EnsureConflict_Requeues(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "fresh-target", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "p"},
	}

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollecttargets"}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(
				_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption,
			) error {
				return apierrors.NewConflict(gr, obj.GetName(), errors.New("resourceVersion conflict"))
			},
		}).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme}

	res, done, err := r.reconcileTargetFinalizers(context.Background(), target.DeepCopy())
	if err != nil {
		t.Fatalf("reconcileTargetFinalizers() error = %v, want nil on ensure-finalizer conflict", err)
	}
	if !done || !res.Requeue { //nolint:staticcheck // SA1019: asserting the reconciler requeues on conflict
		t.Fatalf("reconcileTargetFinalizers() = (res=%+v, done=%v); want done=true, Requeue=true on conflict", res, done)
	}
}
