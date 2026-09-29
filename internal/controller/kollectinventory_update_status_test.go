// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestKollectInventoryReconciler_updateStatus_success(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a", Generation: 4},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("git"),
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).WithStatusSubresource(inv).Build()
	r := &KollectInventoryReconciler{Client: cl}

	result, err := r.updateStatus(context.Background(), inv, 7, perSinkExportOutcome{
		ExportedCount: 1,
		RequeueAfter:  30,
	})
	if err != nil {
		t.Fatalf("updateStatus: %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("expected debounce requeue")
	}

	if inv.Status.ItemCount != 7 || inv.Status.ObservedGeneration != 4 {
		t.Fatalf("status = %#v", inv.Status)
	}

	ready := apimeta.FindStatusCondition(inv.Status.Conditions, conditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("ready condition = %#v", ready)
	}
}

func TestKollectInventoryReconciler_updateStatus_failedWithNoExport(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a", Generation: 4},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("git"),
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).WithStatusSubresource(inv).Build()
	r := &KollectInventoryReconciler{Client: cl}

	apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Exported",
		Message:            "exported",
		ObservedGeneration: inv.Generation,
		LastTransitionTime: metav1.Now(),
	})

	_, err := r.updateStatus(context.Background(), inv, 7, perSinkExportOutcome{
		FailedCount:    1,
		DebouncedCount: 1,
		ExportedCount:  0,
		ExportErr:      errors.New("boom"),
		RequeueAfter:   time.Second,
	})
	if err != nil {
		t.Fatalf("updateStatus: %v", err)
	}

	ready := apimeta.FindStatusCondition(inv.Status.Conditions, conditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != reasonExportFailed || ready.Message != "boom" {
		t.Fatalf("ready condition = %#v", ready)
	}

	degraded := apimeta.FindStatusCondition(inv.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonExportFailed || degraded.Message != "boom" {
		t.Fatalf("degraded condition = %#v", degraded)
	}
}
