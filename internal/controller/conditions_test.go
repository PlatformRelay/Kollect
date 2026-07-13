// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestSetTargetCondition_skipsUnchanged(t *testing.T) {
	t.Parallel()

	fixed := metav1.NewTime(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a", Generation: 2},
		Status: kollectdevv1alpha1.KollectTargetStatus{
			Conditions: []metav1.Condition{{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             "Collecting",
				Message:            "profileRef \"apps\" resolved; collecting 3 resource(s)",
				ObservedGeneration: 2,
				LastTransitionTime: fixed,
			}},
		},
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	bgCtx := context.Background()
	msg := "profileRef \"apps\" resolved; collecting 3 resource(s)"
	if err := setTargetCondition(
		bgCtx, cl, target, 2, &target.Status.Conditions,
		conditionReady, metav1.ConditionTrue, "Collecting", msg,
	); err != nil {
		t.Fatalf("setTargetCondition: %v", err)
	}

	ready := apimeta.FindStatusCondition(target.Status.Conditions, conditionReady)
	if ready == nil {
		t.Fatal("Ready condition missing")
	}
	if !ready.LastTransitionTime.Equal(&fixed) {
		t.Fatalf("LastTransitionTime = %v, want unchanged %v", ready.LastTransitionTime, fixed)
	}
}
