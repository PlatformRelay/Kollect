// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func targetTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	return scheme
}

// TestKollectTargetReconcile_TransientProfileError_RequeuesWithoutDegrade is the
// COV-90-S01 RED lock for the profile-resolve error path. A *transient* (non-NotFound)
// API error while fetching the referenced KollectProfile must PROPAGATE from Reconcile so
// controller-runtime rate-limit-requeues it (backoff), and must NOT hard-degrade the
// target — a transient control-plane blip is a retry, not a permanent config fault.
//
// NOTE (spec vs behaviour): tasks.md phrases this RED as "requeue with backoff +
// Synced=False, Reason=Progressing". The current production Reconcile
// (kollecttarget_controller.go:100-117) returns the raw r.Get error unwrapped and sets NO
// condition on this path (Progressing is used only on export progress elsewhere). Setting
// Synced=Progressing here would be a production behaviour change, which is out of scope for
// this test-only slice. This test therefore locks the ACTUAL contract: error propagates
// (non-nil) AND no Degraded is written. It is non-tautological: it fails if someone swallows
// the error (return nil) or degrades the target on a transient blip.
func TestKollectTargetReconcile_TransientProfileError_RequeuesWithoutDegrade(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "team-target",
			Namespace:  "team-a",
			Generation: 1,
			Finalizers: []string{targetCleanupFinalizer},
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "team-profile"},
	}

	sentinel := errors.New("etcdserver: request timed out")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context, c client.WithWatch, key client.ObjectKey,
				obj client.Object, opts ...client.GetOption,
			) error {
				// Discriminate: only the KollectProfile fetch is transiently broken.
				// The KollectTarget fetch (line 54) must still succeed, else we never
				// reach the profile-resolve branch at line 102.
				if _, ok := obj.(*kollectdevv1alpha1.KollectProfile); ok {
					return sentinel
				}

				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	r := &KollectTargetReconciler{
		Client:   cl,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(5),
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: target.Namespace, Name: target.Name},
	})
	if err == nil {
		t.Fatal("Reconcile() = nil on a transient profile-resolve error; want the error to propagate so controller-runtime requeues with backoff instead of silently succeeding")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Reconcile() error = %v, want the transient profile-resolve error to propagate unchanged", err)
	}

	var got kollectdevv1alpha1.KollectTarget
	if getErr := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got); getErr != nil {
		t.Fatalf("Get target: %v", getErr)
	}
	if cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded); cond != nil && cond.Status == metav1.ConditionTrue {
		t.Fatalf("a transient profile-resolve blip must not hard-degrade the target; got Degraded=%+v", cond)
	}
}

// TestKollectTargetReconcile_Suspended_Degrades covers the spec.suspend branch
// (kollecttarget_controller.go:77-88): a suspended target degrades with reason "Suspended".
func TestKollectTargetReconcile_Suspended_Degrades(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "suspended-target",
			Namespace:  "team-a",
			Generation: 1,
			Finalizers: []string{targetCleanupFinalizer},
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "team-profile", Suspend: true},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: target.Namespace, Name: target.Name},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got); err != nil {
		t.Fatalf("Get target: %v", err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "Suspended" {
		t.Fatalf("Degraded = %+v, want True/Suspended", cond)
	}
}

// TestKollectTargetReconcile_MissingProfileRef_Degrades covers the empty-profileRef branch
// (kollecttarget_controller.go:91-97).
func TestKollectTargetReconcile_MissingProfileRef_Degrades(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "no-profile-target",
			Namespace:  "team-a",
			Generation: 1,
			Finalizers: []string{targetCleanupFinalizer},
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: ""},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: target.Namespace, Name: target.Name},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got); err != nil {
		t.Fatalf("Get target: %v", err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "MissingProfileRef" {
		t.Fatalf("Degraded = %+v, want True/MissingProfileRef", cond)
	}
}

// TestKollectTargetReconcile_ProfileNotFound_Degrades covers the NotFound profile branch
// (kollecttarget_controller.go:103-111): a missing profile hard-degrades with ProfileNotFound.
func TestKollectTargetReconcile_ProfileNotFound_Degrades(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dangling-profile-target",
			Namespace:  "team-a",
			Generation: 1,
			Finalizers: []string{targetCleanupFinalizer},
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "ghost-profile"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: target.Namespace, Name: target.Name},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: target.Namespace, Name: target.Name}, &got); err != nil {
		t.Fatalf("Get target: %v", err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "ProfileNotFound" {
		t.Fatalf("Degraded = %+v, want True/ProfileNotFound", cond)
	}
}

// TestKollectTargetReconcile_TargetGetError_Propagates covers the generic (non-NotFound)
// target Get error branch (kollecttarget_controller.go:63-65).
func TestKollectTargetReconcile_TargetGetError_Propagates(t *testing.T) {
	t.Parallel()

	scheme := targetTestScheme(t)

	sentinel := errors.New("apiserver unavailable")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				_ context.Context, _ client.WithWatch, _ client.ObjectKey,
				obj client.Object, _ ...client.GetOption,
			) error {
				// A non-NotFound error must propagate, not be swallowed by IgnoreNotFound.
				if _, ok := obj.(*kollectdevv1alpha1.KollectTarget); ok {
					return sentinel
				}

				return nil
			},
		}).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "team-target"},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Reconcile() error = %v, want the target Get error to propagate for a rate-limited requeue", err)
	}
}
