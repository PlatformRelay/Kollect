// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestMapEventSinkToInventories(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	eventSink := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats", Namespace: "team-a"},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv1", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			EventSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "nats"}},
		},
	}
	other := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv2", Namespace: "team-a"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv, other).Build()
	r := &KollectInventoryReconciler{Client: cl}

	reqs := r.mapEventSinkToInventories(context.Background(), eventSink)
	if len(reqs) != 1 || reqs[0].Name != "inv1" {
		t.Fatalf("expected one request for inv1, got %#v", reqs)
	}
}

func TestMapEventSinkToInventories_noMatch(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	eventSink := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats", Namespace: "team-a"},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv1", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			EventSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "kafka"}},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()
	r := &KollectInventoryReconciler{Client: cl}

	reqs := r.mapEventSinkToInventories(context.Background(), eventSink)
	if len(reqs) != 0 {
		t.Fatalf("expected no requests, got %#v", reqs)
	}
}
