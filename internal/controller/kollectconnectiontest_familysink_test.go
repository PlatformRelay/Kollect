// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// TestFamilySinkObject_ResolvesEachFamily covers familySinkObject
// (kollectconnectiontest_controller.go:119) across every sink family plus the nil and
// unknown-family guards, and the per-family Get(NotFound) error branch.
func TestFamilySinkObject_ResolvesEachFamily(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	snap := &kollectdevv1alpha1.KollectSnapshotSink{ObjectMeta: metav1.ObjectMeta{Name: "snap", Namespace: "ns"}}
	db := &kollectdevv1alpha1.KollectDatabaseSink{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns"}}
	evt := &kollectdevv1alpha1.KollectEventSink{ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "ns"}}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(snap, db, evt).
		Build()

	r := &KollectConnectionTestReconciler{Client: cl, Scheme: scheme}
	tctx := context.Background()

	cases := []struct {
		name   string
		family string
		obj    string
	}{
		{"snapshot", kollectdevv1alpha1.SinkFamilySnapshot, "snap"},
		{"database", kollectdevv1alpha1.SinkFamilyDatabase, "db"},
		{"event", kollectdevv1alpha1.SinkFamilyEvent, "evt"},
	}
	for _, tc := range cases {
		got, err := r.familySinkObject(tctx, &sink.ResolvedSink{Family: tc.family, Namespace: "ns", Name: tc.obj})
		if err != nil {
			t.Fatalf("%s: familySinkObject() error = %v", tc.name, err)
		}
		if got == nil || got.GetName() != tc.obj {
			t.Fatalf("%s: familySinkObject() = %v, want object %q", tc.name, got, tc.obj)
		}
	}

	// nil resolved sink -> guard error, no panic.
	if _, err := r.familySinkObject(tctx, nil); err == nil {
		t.Fatal("familySinkObject(nil) = nil error; want a guard error")
	}

	// unknown family -> guard error naming nothing to fetch.
	if _, err := r.familySinkObject(tctx, &sink.ResolvedSink{Family: "bogus", Namespace: "ns", Name: "x"}); err == nil {
		t.Fatal("familySinkObject(unknown family) = nil error; want a guard error")
	}

	// owner object already gone -> Get returns NotFound (no panic, error surfaced).
	_, err := r.familySinkObject(tctx, &sink.ResolvedSink{Family: kollectdevv1alpha1.SinkFamilySnapshot, Namespace: "ns", Name: "vanished"})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("familySinkObject(missing) error = %v, want NotFound", err)
	}
}

// TestEnsureOwnerReference_DisabledAndAlreadySet covers the two early-return branches of
// ensureOwnerReference (kollectconnectiontest_controller.go:155-162): OwnerSink explicitly
// disabled, and an ownerReference already pointing at the sink UID.
func TestEnsureOwnerReference_DisabledAndAlreadySet(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "owner-sink", Namespace: "ns", UID: types.UID("sink-uid")},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &KollectConnectionTestReconciler{Client: cl, Scheme: scheme}
	tctx := context.Background()

	// OwnerSink disabled -> no-op, no ownerReference added.
	disabled := false
	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "probe", Namespace: "ns"},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{OwnerSink: &disabled},
	}
	if err := r.ensureOwnerReference(tctx, test, sinkObj); err != nil {
		t.Fatalf("ensureOwnerReference(disabled) error = %v", err)
	}
	if len(test.OwnerReferences) != 0 {
		t.Fatalf("ownerReferences = %#v, want none when OwnerSink is disabled", test.OwnerReferences)
	}

	// Owner reference already present for the sink UID -> no-op, no patch attempted.
	preOwned := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "probe2",
			Namespace:       "ns",
			OwnerReferences: []metav1.OwnerReference{{UID: sinkObj.UID, Name: sinkObj.Name}},
		},
	}
	if err := r.ensureOwnerReference(tctx, preOwned, sinkObj); err != nil {
		t.Fatalf("ensureOwnerReference(already set) error = %v", err)
	}
	if len(preOwned.OwnerReferences) != 1 {
		t.Fatalf("ownerReferences = %#v, want the single existing ref untouched", preOwned.OwnerReferences)
	}
}
