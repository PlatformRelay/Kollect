// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTargetSelectorFor_defaultsToEverything(t *testing.T) {
	t.Parallel()

	sel, err := targetSelectorFor(&kollectdevv1alpha1.KollectClusterInventory{})
	if err != nil {
		t.Fatalf("targetSelectorFor: %v", err)
	}

	if !sel.Matches(labels.Set{"any": "label"}) {
		t.Fatal("nil targetSelector should match everything")
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

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "kollect-system"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git", Endpoint: "https://example.com/repo.git"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	ok, reason, msg := checkClusterInventorySinksReachable(
		context.Background(), cl, "kollect-system", []string{"git"},
	)
	if !ok || reason != reasonSinksReachable || msg == "" {
		t.Fatalf("reachable = %v reason=%q msg=%q", ok, reason, msg)
	}

	ok, reason, _ = checkClusterInventorySinksReachable(
		context.Background(), cl, "kollect-system", []string{"missing"},
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

	_, degraded := r.rollupCounts(inv, targets)
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
			SinkRefs:          kollectdevv1alpha1.NewSinkRefList("git"),
			SinkNamespace:     "kollect-system",
		},
	}
	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "kollect-system"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git", Endpoint: "https://example.com/repo.git"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv, sinkObj).Build()
	r := &KollectClusterInventoryReconciler{Client: cl}

	reqs := r.mapSinkToClusterInventories(context.Background(), sinkObj)
	if len(reqs) != 1 || reqs[0].Name != "platform" {
		t.Fatalf("reqs = %#v", reqs)
	}

	if got := r.mapSinkToClusterInventories(context.Background(), inv); got != nil {
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
