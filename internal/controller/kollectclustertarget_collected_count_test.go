// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// Cluster-target parity for status.collectedCount / collectedCountUpdatedAt (TSP-1, D3):
// the namespaced KollectTarget semantics, driven through the same reconcile status-write
// seam the namespaced suite uses (kollecttarget_collected_count_test.go). The count source
// is the production read the controller already performs (collectedCount → Engine.ItemCount),
// seeded deterministically through the store like kollectclustertarget_unit_test.go — no
// informer machinery is started, so the derived count cannot drift mid-test.
const (
	clusterCountTarget     = "cluster-count"
	clusterCountProfile    = "deployments"
	clusterCountProfileNS  = "kollect-system"
	clusterCountNSReady    = "team-a"
	clusterCountNSChanged  = "team-b"
	clusterCountItemsReady = 17
)

// clusterCountEngine seeds the engine store with n distinct items for the ready namespace.
func clusterCountEngine(t *testing.T, items int) *collect.Engine {
	t.Helper()

	return clusterCountEngineFor(t, clusterCountNSReady, items)
}

// clusterCountEngineFor seeds the engine store with n distinct items under one target
// namespace, so a test controls which namespace the derived count comes from.
func clusterCountEngineFor(t *testing.T, ns string, items int) *collect.Engine {
	t.Helper()

	store := collect.NewStore()
	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	for i := range items {
		store.Upsert(collect.Item{
			TargetNamespace: ns,
			TargetName:      clusterCountTarget,
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       ns,
			Name:            fmt.Sprintf("obj-%d", i),
			Version:         "v1",
			Kind:            "ConfigMap",
		})
	}

	return engine
}

// freshClusterTarget is a target that has never reached Ready: no conditions, no count.
func freshClusterTarget() *kollectdevv1alpha1.KollectClusterTarget {
	return &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: clusterCountTarget, Generation: 2},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
				Name:      clusterCountProfile,
				Namespace: clusterCountProfileNS,
			},
		},
	}
}

// clusterCountClient wraps the shared status-counting fake client around a copy of ct.
func clusterCountClient(
	t *testing.T,
	ct *kollectdevv1alpha1.KollectClusterTarget,
) (client.Client, *int, *kollectdevv1alpha1.KollectClusterTarget) {
	t.Helper()

	cl, updates := clusterTargetStatusClient(t, ct)

	live := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), live); err != nil {
		t.Fatalf("get: %v", err)
	}

	return cl, updates, live
}

// storedClusterTarget reads back what the fake API server kept.
func storedClusterTarget(t *testing.T, cl client.Client) *kollectdevv1alpha1.KollectClusterTarget {
	t.Helper()

	stored := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "", Name: clusterCountTarget}, stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}

	return stored
}

// TestClusterTargetSetReady_persistsCollectedCount is the parity anchor (spec scenario
// "Cluster target persists its count"): when the target reaches Ready while its
// engine-registered synthetic targets hold 17 items, status carries the count and the
// timestamp. Red until T07 wires the write: the controller derives the number for the
// Ready message but never stores it.
func TestClusterTargetSetReady_persistsCollectedCount(t *testing.T) {
	t.Parallel()

	bg := context.Background()

	ct := freshClusterTarget()
	engine := clusterCountEngine(t, clusterCountItemsReady)
	cl, _, live := clusterCountClient(t, ct)

	r := &KollectClusterTargetReconciler{Client: cl, Engine: engine}
	if err := r.setReady(bg, live, []string{clusterCountNSReady}, false); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	stored := storedClusterTarget(t, cl)
	if stored.Status.CollectedCount == nil || *stored.Status.CollectedCount != clusterCountItemsReady {
		t.Fatalf("persisted collectedCount = %v, want %d (the controller derives the number "+
			"for the Ready message but does not store it yet)", stored.Status.CollectedCount, clusterCountItemsReady)
	}
	if stored.Status.CollectedCountUpdatedAt == nil {
		t.Fatal("persisted collectedCountUpdatedAt = <nil>, want set on the first Ready observation")
	}
}

// TestClusterTargetSetReady_persistsCountWhenConditionUnchanged is the PERF-FIX-05 escape
// hatch on the cluster path (spec scenario "Count change is persisted independently of the
// condition text"): a target whose Ready condition already restates the new number — as an
// old binary would have written it — has a byte-identical condition payload, so the shared
// writer skips the write and the count must still reach the API server through the one
// escape-hatch write.
func TestClusterTargetSetReady_persistsCountWhenConditionUnchanged(t *testing.T) {
	t.Parallel()

	bg := context.Background()

	ct := freshClusterTarget()
	ct.Status.ObservedGeneration = 2
	ct.Status.Conditions = []metav1.Condition{{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             reasonCollecting,
		Message:            readyConditionMessage(clusterCountProfile, clusterCountProfileNS, 1, clusterCountItemsReady),
		ObservedGeneration: 2,
	}}
	engine := clusterCountEngine(t, clusterCountItemsReady)
	cl, updates, live := clusterCountClient(t, ct)

	seededCondition := *apimeta.FindStatusCondition(live.Status.Conditions, conditionReady)

	r := &KollectClusterTargetReconciler{Client: cl, Engine: engine}
	if err := r.setReady(bg, live, []string{clusterCountNSReady}, false); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	stored := storedClusterTarget(t, cl)
	if stored.Status.CollectedCount == nil || *stored.Status.CollectedCount != clusterCountItemsReady {
		t.Fatalf("persisted collectedCount = %v, want %d (a byte-identical condition payload "+
			"must not swallow the count)", stored.Status.CollectedCount, clusterCountItemsReady)
	}
	if stored.Status.CollectedCountUpdatedAt == nil {
		t.Fatal("persisted collectedCountUpdatedAt = <nil>, want set")
	}

	kept := apimeta.FindStatusCondition(stored.Status.Conditions, conditionReady)
	if kept == nil || kept.Message != seededCondition.Message ||
		!kept.LastTransitionTime.Equal(&seededCondition.LastTransitionTime) {
		t.Fatalf("Ready condition was rewritten (%+v), want the seeded payload kept byte-identical", kept)
	}
	if *updates != 1 {
		t.Fatalf("status writes = %d, want 1 (the escape hatch, not the condition writer)", *updates)
	}
}

// TestClusterTargetSetReady_steadyCountKeepsTimestamp covers the spec scenario "Steady
// count keeps its timestamp": two consecutive reconciles deriving the same count leave the
// timestamp where the first one put it, and the second reconcile issues no write at all.
func TestClusterTargetSetReady_steadyCountKeepsTimestamp(t *testing.T) {
	t.Parallel()

	bg := context.Background()

	ct := freshClusterTarget()
	engine := clusterCountEngine(t, clusterCountItemsReady)
	cl, updates, live := clusterCountClient(t, ct)

	r := &KollectClusterTargetReconciler{Client: cl, Engine: engine}
	if err := r.setReady(bg, live, []string{clusterCountNSReady}, false); err != nil {
		t.Fatalf("setReady (first reconcile): %v", err)
	}

	first := storedClusterTarget(t, cl)
	if first.Status.CollectedCount == nil || first.Status.CollectedCountUpdatedAt == nil {
		t.Fatalf("first reconcile persisted collectedCount=%v collectedCountUpdatedAt=%v, want both set",
			first.Status.CollectedCount, first.Status.CollectedCountUpdatedAt)
	}
	firstStamp := *first.Status.CollectedCountUpdatedAt
	writesAfterFirst := *updates

	if err := r.setReady(bg, live, []string{clusterCountNSReady}, false); err != nil {
		t.Fatalf("setReady (second reconcile): %v", err)
	}

	second := storedClusterTarget(t, cl)
	if second.Status.CollectedCount == nil || *second.Status.CollectedCount != clusterCountItemsReady {
		t.Fatalf("steady count = %v, want unchanged %d", second.Status.CollectedCount, clusterCountItemsReady)
	}
	if second.Status.CollectedCountUpdatedAt == nil ||
		!second.Status.CollectedCountUpdatedAt.Equal(&firstStamp) {
		t.Fatalf("steady timestamp = %v, want unchanged %v", second.Status.CollectedCountUpdatedAt, firstStamp)
	}
	if *updates != writesAfterFirst {
		t.Fatalf("status writes grew from %d to %d on a steady count, want no second write",
			writesAfterFirst, *updates)
	}
}

// TestClusterTargetSetReady_persistsCountAndFilterInOneWrite is the spec scenario "Count
// and filter move together, one write": the matched namespace swaps a→b while the count
// stays 17 and the condition payload restates it byte-identically, so one status write —
// the existing filter-status escape hatch, extended by T07 to `filterChanged ||
// countChanged` — must persist both; no second Status().Update site.
func TestClusterTargetSetReady_persistsCountAndFilterInOneWrite(t *testing.T) {
	t.Parallel()

	bg := context.Background()

	ct := freshClusterTarget()
	ct.Status.ObservedGeneration = 2
	ct.Status.CollectionFilterStatus = kollectdevv1alpha1.CollectionFilterStatus{
		MatchedNamespaces:   []string{clusterCountNSReady},
		EffectiveNamespaces: []string{clusterCountNSReady},
	}
	ct.Status.Conditions = []metav1.Condition{{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             reasonCollecting,
		Message:            readyConditionMessage(clusterCountProfile, clusterCountProfileNS, 1, clusterCountItemsReady),
		ObservedGeneration: 2,
	}}
	engine := clusterCountEngineFor(t, clusterCountNSChanged, clusterCountItemsReady)
	cl, updates, live := clusterCountClient(t, ct)

	// The predicate is computed before the mutation, exactly as Reconcile does.
	filterChanged := clusterTargetFilterChanged(
		live, []string{clusterCountNSChanged}, []string{clusterCountNSChanged}, 0,
	)
	if !filterChanged {
		t.Fatal("fixture did not produce a filter change")
	}
	updateClusterTargetFilterStatus(
		live, []string{clusterCountNSChanged}, []string{clusterCountNSChanged}, 0,
	)

	r := &KollectClusterTargetReconciler{Client: cl, Engine: engine}
	if err := r.setReady(bg, live, []string{clusterCountNSChanged}, filterChanged); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	stored := storedClusterTarget(t, cl)
	if *updates != 1 {
		t.Fatalf("status writes = %d, want 1 (one write for filter status and count together)", *updates)
	}
	if !slices.Equal(stored.Status.MatchedNamespaces, []string{clusterCountNSChanged}) {
		t.Fatalf("persisted matchedNamespaces = %v, want [%s]", stored.Status.MatchedNamespaces, clusterCountNSChanged)
	}
	if stored.Status.CollectedCount == nil || *stored.Status.CollectedCount != clusterCountItemsReady {
		t.Fatalf("persisted collectedCount = %v, want %d in the same write",
			stored.Status.CollectedCount, clusterCountItemsReady)
	}
	if stored.Status.CollectedCountUpdatedAt == nil {
		t.Fatal("persisted collectedCountUpdatedAt = <nil>, want set")
	}
}

// TestClusterTargetSetDegraded_keepsLastCount is the spec scenario "Degraded target keeps
// its last count": a target that was Ready and counted 17 goes Degraded, and the write must
// not clear the count or move its timestamp. The guard passes before T07 as well (the write
// path never touches the fields) and pins the Degraded branch of the T07 implementation.
func TestClusterTargetSetDegraded_keepsLastCount(t *testing.T) {
	t.Parallel()

	bg := context.Background()

	stale := metav1.NewTime(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	count := int64(clusterCountItemsReady)
	ct := freshClusterTarget()
	ct.Status.ObservedGeneration = 2
	ct.Status.CollectedCount = &count
	ct.Status.CollectedCountUpdatedAt = &stale
	ct.Status.CollectionFilterStatus = kollectdevv1alpha1.CollectionFilterStatus{
		MatchedNamespaces:   []string{clusterCountNSReady},
		EffectiveNamespaces: []string{clusterCountNSReady},
	}
	ct.Status.Conditions = []metav1.Condition{{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             reasonCollecting,
		Message:            readyConditionMessage(clusterCountProfile, clusterCountProfileNS, 1, clusterCountItemsReady),
		ObservedGeneration: 2,
	}}
	cl, _, live := clusterCountClient(t, ct)

	r := &KollectClusterTargetReconciler{Client: cl}
	if err := r.setDegraded(bg, live, "SimulatedDegradation", "synthetic failure for the parity test", false); err != nil {
		t.Fatalf("setDegraded: %v", err)
	}

	stored := storedClusterTarget(t, cl)
	degraded := apimeta.FindStatusCondition(stored.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Reason != "SimulatedDegradation" {
		t.Fatalf("Degraded = %+v, want reason SimulatedDegradation", degraded)
	}
	if stored.Status.CollectedCount == nil || *stored.Status.CollectedCount != clusterCountItemsReady {
		t.Fatalf("collectedCount after Degraded = %v, want last known %d kept",
			stored.Status.CollectedCount, clusterCountItemsReady)
	}
	if stored.Status.CollectedCountUpdatedAt == nil ||
		!stored.Status.CollectedCountUpdatedAt.Equal(&stale) {
		t.Fatalf("collectedCountUpdatedAt after Degraded = %v, want unchanged %v",
			stored.Status.CollectedCountUpdatedAt, stale)
	}
}
