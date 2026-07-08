// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func newScopeTestTarget() *kollectdevv1alpha1.KollectTarget {
	return &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "scope-test-target",
			Namespace:  "scope-test-ns",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "scope-test-profile"},
	}
}

func newScopeTestReconciler(t *testing.T, target *kollectdevv1alpha1.KollectTarget) (*KollectTargetReconciler, *record.FakeRecorder) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target).WithStatusSubresource(target).Build()
	recorder := record.NewFakeRecorder(5)

	return &KollectTargetReconciler{Client: fakeClient, Recorder: recorder}, recorder
}

// EC-P2-01: a forbidden scope (RBAC denied for one or more scoped namespaces)
// must degrade scope, not the whole target — Ready stays true, Synced carries
// the forbidden-scope reason, and a Warning event is recorded.
func TestApplyTargetReadyState_ForbiddenScope_DegradesScopeNotWholeTarget(t *testing.T) {
	t.Parallel()

	target := newScopeTestTarget()
	r, recorder := newScopeTestReconciler(t, target)

	if _, err := r.applyTargetReadyState(context.Background(), target, 3, true, false, 0, ""); err != nil {
		t.Fatalf("applyTargetReadyState: %v", err)
	}

	ready := apimeta.FindStatusCondition(target.Status.Conditions, conditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True (forbidden scope must not hard-fail the whole target)", ready)
	}

	degraded := apimeta.FindStatusCondition(target.Status.Conditions, conditionDegraded)
	if degraded != nil {
		t.Fatalf("Degraded condition = %#v, want absent for a scope-only forbidden error", degraded)
	}

	synced := apimeta.FindStatusCondition(target.Status.Conditions, conditionSynced)
	if synced == nil || synced.Reason != reasonScopeForbidden {
		t.Fatalf("Synced condition = %#v, want reason %q", synced, reasonScopeForbidden)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonScopeForbidden) {
			t.Fatalf("event = %q, want it to mention %q", ev, reasonScopeForbidden)
		}
	default:
		t.Fatal("expected a Warning event for the forbidden scope")
	}
}

// Baseline regression: with no forbidden scope, Synced keeps reason "Collecting".
func TestApplyTargetReadyState_NoIssues_SyncedReasonCollecting(t *testing.T) {
	t.Parallel()

	target := newScopeTestTarget()
	r, _ := newScopeTestReconciler(t, target)

	if _, err := r.applyTargetReadyState(context.Background(), target, 2, false, false, 0, ""); err != nil {
		t.Fatalf("applyTargetReadyState: %v", err)
	}

	ready := apimeta.FindStatusCondition(target.Status.Conditions, conditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", ready)
	}

	synced := apimeta.FindStatusCondition(target.Status.Conditions, conditionSynced)
	if synced == nil || synced.Reason != "Collecting" {
		t.Fatalf("Synced condition = %#v, want reason %q", synced, "Collecting")
	}
}

// Access-check API failures are a different error class (transient API outage,
// not a scoped RBAC denial) and must keep hard-failing the whole target.
func TestApplyTargetReadyState_AccessCheckFailure_StillFullyDegrades(t *testing.T) {
	t.Parallel()

	target := newScopeTestTarget()
	r, _ := newScopeTestReconciler(t, target)

	if _, err := r.applyTargetReadyState(context.Background(), target, 0, false, true, 0, ""); err != nil {
		t.Fatalf("applyTargetReadyState: %v", err)
	}

	degraded := apimeta.FindStatusCondition(target.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != "AccessCheckFailed" {
		t.Fatalf("Degraded condition = %#v, want True/AccessCheckFailed", degraded)
	}

	ready := apimeta.FindStatusCondition(target.Status.Conditions, conditionReady)
	if ready != nil {
		t.Fatalf("Ready condition = %#v, want absent when access checks fail", ready)
	}
}
