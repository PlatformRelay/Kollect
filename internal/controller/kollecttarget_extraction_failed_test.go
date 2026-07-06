// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EC-P1-05: extraction failures (invalid CEL/JSONPath or per-resource evaluation error) are
// GUIDELINES.md §1 ErrTerminal and must hard-degrade the target (unlike ErrForbidden/EC-P2-01,
// which only degrades scope) — and the failure count + last message must be visible in status.
func TestApplyTargetReadyState_ExtractionFailures_DegradesAndSurfacesStatus(t *testing.T) {
	t.Parallel()

	target := newScopeTestTarget()
	r, recorder := newScopeTestReconciler(t, target)

	if _, err := r.applyTargetReadyState(
		context.Background(), target, 3, false, false, 2, "attribute \"bad\": compile CEL: boom",
	); err != nil {
		t.Fatalf("applyTargetReadyState: %v", err)
	}

	degraded := apimeta.FindStatusCondition(target.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonExtractionFailed {
		t.Fatalf("Degraded condition = %#v, want True/%s", degraded, reasonExtractionFailed)
	}

	ready := apimeta.FindStatusCondition(target.Status.Conditions, conditionReady)
	if ready != nil {
		t.Fatalf("Ready condition = %#v, want absent when extraction fails", ready)
	}

	if target.Status.ExtractionFailures != 2 {
		t.Fatalf("status.ExtractionFailures = %d, want 2", target.Status.ExtractionFailures)
	}
	if target.Status.LastExtractionError == "" {
		t.Fatal("status.LastExtractionError = \"\", want the extraction error message")
	}

	select {
	case ev := <-recorder.Events:
		if ev == "" {
			t.Fatal("expected a non-empty event")
		}
	default:
		t.Fatal("expected a Warning event for the extraction failure")
	}
}

// Baseline: with no extraction failures, status fields stay clear and Ready holds.
func TestApplyTargetReadyState_NoExtractionFailures_StatusFieldsClear(t *testing.T) {
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

	if target.Status.ExtractionFailures != 0 {
		t.Fatalf("status.ExtractionFailures = %d, want 0", target.Status.ExtractionFailures)
	}
	if target.Status.LastExtractionError != "" {
		t.Fatalf("status.LastExtractionError = %q, want empty", target.Status.LastExtractionError)
	}
}
