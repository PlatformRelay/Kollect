// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConnectionTestTTL(t *testing.T) {
	t.Parallel()

	defaultTTL := connectionTestTTL(&kollectdevv1alpha1.KollectConnectionTest{})
	if defaultTTL != 300*time.Second {
		t.Fatalf("default TTL = %v", defaultTTL)
	}

	custom := int32(60)
	ttl := connectionTestTTL(&kollectdevv1alpha1.KollectConnectionTest{
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{TTLSecondsAfterFinished: &custom},
	})
	if ttl != 60*time.Second {
		t.Fatalf("custom TTL = %v", ttl)
	}

	negative := int32(-5)
	ttl = connectionTestTTL(&kollectdevv1alpha1.KollectConnectionTest{
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{TTLSecondsAfterFinished: &negative},
	})
	if ttl != 0 {
		t.Fatalf("negative TTL = %v", ttl)
	}
}

func TestKollectConnectionTestReconciler_sinkNotFound(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "probe-missing",
			Namespace:  "team-a",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: kollectdevv1alpha1.ConnectionTestSinkRef{DatabaseSinkRef: "missing-sink"}},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(test).
		WithObjects(test).
		Build()

	r := &KollectConnectionTestReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: test.Namespace, Name: test.Name},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectConnectionTest
	nn := types.NamespacedName{Namespace: test.Namespace, Name: test.Name}
	if err := c.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}

	if !got.Status.Completed {
		t.Fatal("expected completed status")
	}

	cond := apimeta.FindStatusCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != reasonSinkNotFound {
		t.Fatalf("ConnectionVerified = %+v", cond)
	}
}

func TestKollectConnectionTestReconciler_probeFailed(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-git", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "://invalid",
			},
		},
	}
	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "probe-bad",
			Namespace:  "team-a",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{
			SinkRef: kollectdevv1alpha1.ConnectionTestSinkRef{SnapshotSinkRef: sinkObj.Name},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(test, sinkObj).
		WithObjects(test, sinkObj).
		Build()

	r := &KollectConnectionTestReconciler{Client: c, Scheme: scheme}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: test.Namespace, Name: test.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectConnectionTest
	nn := types.NamespacedName{Namespace: test.Namespace, Name: test.Name}
	if err := c.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}

	cond := apimeta.FindStatusCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "ConnectionTestFailed" {
		t.Fatalf("ConnectionVerified = %+v", cond)
	}
}

func TestKollectConnectionTestReconciler_ownerReference(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team-git",
			Namespace: "team-a",
			UID:       types.UID("sink-uid"),
		},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "://invalid",
			},
		},
	}
	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "probe-owner",
			Namespace:  "team-a",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{
			SinkRef: kollectdevv1alpha1.ConnectionTestSinkRef{SnapshotSinkRef: sinkObj.Name},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(test, sinkObj).
		WithObjects(test, sinkObj).
		Build()

	r := &KollectConnectionTestReconciler{Client: c, Scheme: scheme}
	_, _ = r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: test.Namespace, Name: test.Name},
	})

	var got kollectdevv1alpha1.KollectConnectionTest
	nn := types.NamespacedName{Namespace: test.Namespace, Name: test.Name}
	if err := c.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}

	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != sinkObj.UID {
		t.Fatalf("ownerReferences = %#v", got.OwnerReferences)
	}
}

func TestKollectConnectionTestReconciler_setProbeSucceeded(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "probe-ok", Namespace: "team-a", Generation: 2},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: kollectdevv1alpha1.ConnectionTestSinkRef{DatabaseSinkRef: "demo"}},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(test).
		WithObjects(test).
		Build()

	r := &KollectConnectionTestReconciler{Client: c, Scheme: scheme}
	if _, err := r.setProbeSucceeded(context.Background(), test, "ok"); err != nil {
		t.Fatal(err)
	}

	cond := apimeta.FindStatusCondition(test.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("ConnectionVerified = %+v", cond)
	}
}

func TestKollectConnectionTestReconciler_reconcileTTLDeletes(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	zero := int32(0)
	completedAt := metav1.NewTime(time.Now().Add(-time.Minute))
	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "probe-ttl", Namespace: "team-a", Generation: 1},
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{
			SinkRef:                 kollectdevv1alpha1.ConnectionTestSinkRef{DatabaseSinkRef: "any"},
			TTLSecondsAfterFinished: &zero,
		},
		Status: kollectdevv1alpha1.KollectConnectionTestStatus{
			Completed:          true,
			ObservedGeneration: 1,
			CompletedAt:        &completedAt,
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionConnectionVerified,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(test).
		WithObjects(test).
		Build()

	r := &KollectConnectionTestReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: test.Namespace, Name: test.Name},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	nn := types.NamespacedName{Namespace: test.Namespace, Name: test.Name}
	err = c.Get(context.Background(), nn, &kollectdevv1alpha1.KollectConnectionTest{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound after TTL delete, got %v", err)
	}
}
