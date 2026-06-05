// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectInventoryReconciler_setInventoryDegraded(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a", Generation: 3},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SinkRefs: []string{"git"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).WithStatusSubresource(inv).Build()
	r := &KollectInventoryReconciler{Client: cl}

	result, err := r.setInventoryDegraded(context.Background(), inv, 5, "SpillRequired", "needs object store")
	if err != nil {
		t.Fatalf("setInventoryDegraded: %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("expected debounce requeue")
	}

	cond := apimeta.FindStatusCondition(inv.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "SpillRequired" {
		t.Fatalf("degraded condition = %#v", cond)
	}
	if inv.Status.ItemCount != 5 || inv.Status.ObservedGeneration != 3 {
		t.Fatalf("status = %#v", inv.Status)
	}
}

func TestKollectInventoryReconciler_mapSinkToInventories(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	invMatch := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: []string{"git", "s3"}},
	}
	invOther := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: []string{"kafka"}},
	}
	sink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(invMatch, invOther, sink).Build()
	r := &KollectInventoryReconciler{Client: cl}

	reqs := r.mapSinkToInventories(context.Background(), sink)
	if len(reqs) != 1 || reqs[0].Namespace != "team-a" || reqs[0].Name != "platform" {
		t.Fatalf("reqs = %#v", reqs)
	}

	if got := r.mapSinkToInventories(context.Background(), invMatch); got != nil {
		t.Fatalf("non-sink object should return nil, got %#v", got)
	}
}
