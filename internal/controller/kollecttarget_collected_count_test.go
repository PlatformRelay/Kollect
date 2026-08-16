// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// collectedCountFixture builds a target that already reached Ready reporting `stored`
// resources, together with a fake client holding it — the state a long-running target
// is in when objects start entering or leaving its matched set.
func collectedCountFixture(t *testing.T, stored int64, lastUpdate metav1.Time) (
	*kollectdevv1alpha1.KollectTarget, client.Client,
) {
	t.Helper()

	count := stored
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a", Generation: 2},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "apps"},
		Status: kollectdevv1alpha1.KollectTargetStatus{
			ObservedGeneration:      2,
			CollectedCount:          &count,
			CollectedCountUpdatedAt: &lastUpdate,
			Conditions: []metav1.Condition{{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             reasonCollecting,
				Message:            readyMessageFor(stored),
				ObservedGeneration: 2,
				LastTransitionTime: lastUpdate,
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

	return target, cl
}

func readyMessageFor(count int64) string {
	return fmt.Sprintf("profileRef %q resolved; collecting %d resource(s)", "apps", count)
}

// PERF-FIX-05 / F-05: a target whose matched set grew must persist the new number.
// setTargetCondition skips the API write for an unchanged Ready condition, so this is
// the regression guard for "the count changed but nothing reached the API server".
func TestSetReadyPersistsGrowingCollectedCount(t *testing.T) {
	t.Parallel()

	stale := metav1.NewTime(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	target, cl := collectedCountFixture(t, 1000, stale)

	r := &KollectTargetReconciler{Client: cl}
	result, err := r.setReady(context.Background(), target, 1200, "", "", "", "")
	if err != nil {
		t.Fatalf("setReady: %v", err)
	}
	if result.RequeueAfter != DefaultTargetCountResync {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, DefaultTargetCountResync)
	}

	var stored kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(target), &stored); err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Status.CollectedCount == nil || *stored.Status.CollectedCount != 1200 {
		t.Fatalf("persisted collectedCount = %v, want 1200", stored.Status.CollectedCount)
	}
	if stored.Status.CollectedCountUpdatedAt == nil || !stored.Status.CollectedCountUpdatedAt.After(stale.Time) {
		t.Fatalf("collectedCountUpdatedAt = %v, want newer than %v", stored.Status.CollectedCountUpdatedAt, stale)
	}
	if !strings.Contains(stored.Status.Conditions[0].Message, "collecting 1200 resource(s)") {
		t.Fatalf("Ready message = %q, want it to restate the live count", stored.Status.Conditions[0].Message)
	}
}

// Objects leaving the selector must shrink the number, including all the way to zero —
// which is why collectedCount is a pointer: an absent field would be indistinguishable
// from "never measured".
func TestSetReadyPersistsShrinkingCollectedCount(t *testing.T) {
	t.Parallel()

	stale := metav1.NewTime(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	target, cl := collectedCountFixture(t, 1200, stale)

	r := &KollectTargetReconciler{Client: cl}
	if _, err := r.setReady(context.Background(), target, 0, "", "", "", ""); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	var stored kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(target), &stored); err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Status.CollectedCount == nil {
		t.Fatal("collectedCount is nil; a measured zero must be distinguishable from unmeasured")
	}
	if *stored.Status.CollectedCount != 0 {
		t.Fatalf("persisted collectedCount = %d, want 0", *stored.Status.CollectedCount)
	}
}

// An unchanged count must not churn the timestamp: collectedCountUpdatedAt means "when
// the number last moved", and refreshing it every resync would make a frozen count look
// live — the exact failure this story exists to remove.
func TestSyncCollectedCountKeepsTimestampWhenUnchanged(t *testing.T) {
	t.Parallel()

	stale := metav1.NewTime(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	count := int64(1000)
	target := &kollectdevv1alpha1.KollectTarget{
		Status: kollectdevv1alpha1.KollectTargetStatus{
			CollectedCount:          &count,
			CollectedCountUpdatedAt: &stale,
		},
	}

	if got := syncCollectedCount(target, 1000); got != 1000 {
		t.Fatalf("syncCollectedCount = %d, want 1000", got)
	}
	if !target.Status.CollectedCountUpdatedAt.Equal(&stale) {
		t.Fatalf("timestamp = %v, want unchanged %v", target.Status.CollectedCountUpdatedAt, stale)
	}
}

// A target the controller has never counted reports nothing rather than a misleading zero.
func TestSyncCollectedCountFirstObservationSetsTimestamp(t *testing.T) {
	t.Parallel()

	target := &kollectdevv1alpha1.KollectTarget{}
	if target.Status.CollectedCount != nil {
		t.Fatal("fresh target must not carry a count")
	}

	if got := syncCollectedCount(target, 3); got != 3 {
		t.Fatalf("syncCollectedCount = %d, want 3", got)
	}
	if target.Status.CollectedCount == nil || *target.Status.CollectedCount != 3 {
		t.Fatalf("collectedCount = %v, want 3", target.Status.CollectedCount)
	}
	if target.Status.CollectedCountUpdatedAt == nil {
		t.Fatal("collectedCountUpdatedAt must be set on the first observation")
	}
}
