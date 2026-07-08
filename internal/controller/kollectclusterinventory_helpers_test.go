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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestTargetSelectorFor_defaultsToNil(t *testing.T) {
	t.Parallel()

	sel, err := targetSelectorFor(&kollectdevv1alpha1.KollectClusterInventory{})
	if err != nil {
		t.Fatalf("targetSelectorFor: %v", err)
	}

	if sel != nil {
		t.Fatal("nil targetSelector should return nil selector")
	}
}

func TestTargetIncluded_respectsTargetRefs(t *testing.T) {
	t.Parallel()

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			TargetRefs: []string{"alpha", "beta"},
		},
	}

	if !targetIncluded(inv, &kollectdevv1alpha1.KollectClusterTarget{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}}) {
		t.Fatal("expected alpha to be included")
	}

	if targetIncluded(inv, &kollectdevv1alpha1.KollectClusterTarget{ObjectMeta: metav1.ObjectMeta{Name: "gamma"}}) {
		t.Fatal("expected gamma to be excluded")
	}

	inv.Spec.TargetRefs = nil
	if !targetIncluded(inv, &kollectdevv1alpha1.KollectClusterTarget{ObjectMeta: metav1.ObjectMeta{Name: "any"}}) {
		t.Fatal("empty targetRefs should include all targets")
	}
}

func TestPerSinkCoalesceTracker_nextDue(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "platform"
	sinkName := "git"
	now := time.Now().UTC().Truncate(time.Second)
	tracker.record(invKey, sinkName, 1, "abc", now)

	got := tracker.nextDue(invKey, sinkName, time.Minute, now)
	if got <= 0 {
		t.Fatalf("nextDue = %v", got)
	}
}

func TestClusterTargetReady(t *testing.T) {
	t.Parallel()

	ready := &kollectdevv1alpha1.KollectClusterTarget{}
	apimeta.SetStatusCondition(&ready.Status.Conditions, metav1.Condition{
		Type:   conditionReady,
		Status: metav1.ConditionTrue,
	})
	if !clusterTargetReady(ready) {
		t.Fatal("expected ready cluster target")
	}

	notReady := &kollectdevv1alpha1.KollectClusterTarget{}
	if clusterTargetReady(notReady) {
		t.Fatal("expected not-ready cluster target")
	}
}

func TestCheckClusterInventorySinksReachable(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	ok, reason, msg := checkClusterInventorySinksReachable(
		context.Background(), cl, "kollect-system", []kollectdevv1alpha1.InventorySinkBinding{
			{Name: "git", Family: kollectdevv1alpha1.SinkFamilyDatabase},
		},
	)
	if !ok || reason != reasonSinksReachable || msg == "" {
		t.Fatalf("reachable = %v reason=%q msg=%q", ok, reason, msg)
	}

	ok, reason, _ = checkClusterInventorySinksReachable(
		context.Background(), cl, "kollect-system", []kollectdevv1alpha1.InventorySinkBinding{{Name: "missing", Family: kollectdevv1alpha1.SinkFamilyDatabase}},
	)
	if ok || reason != reasonSinkNotFound {
		t.Fatalf("missing sink = %v reason=%q", ok, reason)
	}
}

func TestRollupCounts_marksNotReadyTargets(t *testing.T) {
	t.Parallel()

	r := &KollectClusterInventoryReconciler{}
	inv := &kollectdevv1alpha1.KollectClusterInventory{}
	targets := []kollectdevv1alpha1.KollectClusterTarget{
		{ObjectMeta: metav1.ObjectMeta{Name: "ready"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pending"}},
	}
	apimeta.SetStatusCondition(&targets[0].Status.Conditions, metav1.Condition{
		Type:   conditionReady,
		Status: metav1.ConditionTrue,
	})

	_, degraded := r.rollupCounts(inv, targets, nil)
	if len(degraded) != 1 || degraded[0] != "pending" {
		t.Fatalf("degraded = %v, want [pending]", degraded)
	}
}

func TestMapClusterTargetToInventories(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()
	r := &KollectClusterInventoryReconciler{Client: cl}

	reqs := r.mapClusterTargetToInventories(context.Background(), &kollectdevv1alpha1.KollectClusterTarget{})
	if len(reqs) != 1 || reqs[0].Name != "platform" {
		t.Fatalf("reqs = %#v", reqs)
	}
}

func TestMapSinkToClusterInventories(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
			DatabaseSinkRefs:  kollectdevv1alpha1.NewSinkRefList("git"),
			SinkNamespace:     "kollect-system",
		},
	}
	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "kollect-system"},
		Spec:       kollectdevv1alpha1.KollectDatabaseSinkSpec{},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithIndex(
			&kollectdevv1alpha1.KollectClusterInventory{},
			clusterInventorySinkFieldIndex, indexClusterInventorySinkBindings,
		).
		WithObjects(inv, sinkObj).Build()
	r := &KollectClusterInventoryReconciler{Client: cl}

	reqs := r.mapClusterDatabaseSinkToInventories(context.Background(), sinkObj)
	if len(reqs) != 1 || reqs[0].Name != "platform" {
		t.Fatalf("reqs = %#v", reqs)
	}

	if got := r.mapClusterDatabaseSinkToInventories(context.Background(), inv); len(got) != 0 {
		t.Fatalf("non-sink object should return nil, got %#v", got)
	}
}

func TestKollectClusterInventoryReconciler_setDegraded(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Generation: 2},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).WithStatusSubresource(inv).Build()
	r := &KollectClusterInventoryReconciler{Client: cl}

	result, err := r.setDegraded(context.Background(), inv, "ExportFailed", "sink down")
	if err != nil {
		t.Fatalf("setDegraded: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Fatal("expected requeue after debounce")
	}

	cond := apimeta.FindStatusCondition(inv.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "ExportFailed" {
		t.Fatalf("degraded condition = %+v", cond)
	}

	var stored kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "platform"}, &stored); err != nil {
		t.Fatal(err)
	}

	if stored.Status.ObservedGeneration != 2 {
		t.Fatalf("observed generation = %d", stored.Status.ObservedGeneration)
	}
}

func TestKollectClusterInventoryReconciler_composeNamespaceRollup(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "target-a",
		UID:             "uid-a1",
		Namespace:       "team-a",
		Name:            "cfg-a1",
		Version:         "v1",
		Kind:            "ConfigMap",
	})
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "target-b",
		UID:             "uid-a2",
		Namespace:       "team-a",
		Name:            "cfg-a2",
		Version:         "v1",
		Kind:            "ConfigMap",
	})
	store.Upsert(collect.Item{
		TargetNamespace: "team-b",
		TargetName:      "target-a",
		UID:             "uid-b1",
		Namespace:       "team-b",
		Name:            "cfg-b1",
		Version:         "v1",
		Kind:            "ConfigMap",
	})

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces("target-a", []string{"team-a", "team-b"})
	engine.BindClusterTargetNamespaces("target-b", []string{"team-a"})

	reconciler := &KollectClusterInventoryReconciler{
		Store:  store,
		Engine: engine,
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			Dedupe: kollectdevv1alpha1.ClusterInventoryDedupeKeepAll,
		},
	}
	targets := []kollectdevv1alpha1.KollectClusterTarget{
		{ObjectMeta: metav1.ObjectMeta{Name: "target-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "target-b"}},
	}

	rollup, err := reconciler.composeNamespaceRollup(inv, targets, nil)
	if err != nil {
		t.Fatalf("composeNamespaceRollup: %v", err)
	}

	if len(rollup.Items) != 3 {
		t.Fatalf("rollup item count = %d, want 3", len(rollup.Items))
	}
	if len(rollup.Payload) == 0 {
		t.Fatal("payload is empty")
	}
	if rollup.Checksum == "" {
		t.Fatal("rollup checksum is empty")
	}
	if len(rollup.NamespaceShards) != 2 {
		t.Fatalf("namespace shards = %d, want 2", len(rollup.NamespaceShards))
	}
	if rollup.NamespaceShards[0].Namespace != "team-a" || rollup.NamespaceShards[1].Namespace != "team-b" {
		t.Fatalf("namespace shard order = %#v, want [team-a team-b]", rollup.NamespaceShards)
	}
	if rollup.NamespaceShards[0].ItemCount != 2 || rollup.NamespaceShards[0].TargetCount != 2 {
		t.Fatalf("team-a shard = %#v, want itemCount=2 targetCount=2", rollup.NamespaceShards[0])
	}
	if rollup.NamespaceShards[1].ItemCount != 1 || rollup.NamespaceShards[1].TargetCount != 1 {
		t.Fatalf("team-b shard = %#v, want itemCount=1 targetCount=1", rollup.NamespaceShards[1])
	}
}
