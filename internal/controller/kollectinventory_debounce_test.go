// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestPerSinkCoalesceTracker(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "default/team-inventory"
	sinkName := "postgres"
	interval := 30 * time.Second
	now := time.Now()
	gen := int64(1)
	hashA := "fingerprint-a"
	hashB := "fingerprint-b"

	if tracker.shouldSkip(invKey, sinkName, gen, hashA, interval, now) {
		t.Fatal("first export must not debounce")
	}

	tracker.record(invKey, sinkName, gen, hashA, now)
	if !tracker.shouldSkip(invKey, sinkName, gen, hashA, interval, now) {
		t.Fatal("identical payload within interval should debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, gen, hashB, interval, now) {
		t.Fatal("payload change must bypass debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, gen+1, hashA, interval, now) {
		t.Fatal("spec generation bump must bypass debounce")
	}
}

// TestNamespaceFingerprintCache_SkipsRecomputeWhenVersionUnchanged backs AR-10
// (PERF-01 remainder): the cache must serve a memoized fingerprint without
// invoking the (expensive, full-namespace-snapshot) compute function again as
// long as the namespace's Store mutation version hasn't moved — and must
// invoke it again the moment the version does move.
func TestNamespaceFingerprintCache_SkipsRecomputeWhenVersionUnchanged(t *testing.T) {
	t.Parallel()

	var c namespaceFingerprintCache
	calls := 0
	compute := func() string {
		calls++

		return fmt.Sprintf("fp-call-%d", calls)
	}

	fp1 := c.getOrCompute("ns-a", 1, compute)
	if calls != 1 {
		t.Fatalf("first call: calls = %d, want 1", calls)
	}

	fp2 := c.getOrCompute("ns-a", 1, compute)
	if calls != 1 {
		t.Fatalf("same version: calls = %d, want still 1 (cache hit)", calls)
	}
	if fp2 != fp1 {
		t.Fatalf("same version returned different fingerprint: %q vs %q", fp1, fp2)
	}

	fp3 := c.getOrCompute("ns-a", 2, compute)
	if calls != 2 {
		t.Fatalf("version bumped: calls = %d, want 2 (recompute)", calls)
	}
	if fp3 == fp1 {
		t.Fatalf("version bumped but fingerprint unchanged: %q", fp3)
	}

	// A different namespace must not share cache state with ns-a.
	fp4 := c.getOrCompute("ns-b", 1, compute)
	if calls != 3 {
		t.Fatalf("different namespace: calls = %d, want 3 (no shared cache)", calls)
	}
	if fp4 == fp3 {
		t.Fatalf("different namespace returned same fingerprint as ns-a: %q", fp4)
	}
}

func TestKollectInventoryReconciler_exportDebounce_perInventory(t *testing.T) {
	t.Parallel()

	interval := metav1.Duration{Duration: 5 * time.Second}
	rec := &KollectInventoryReconciler{}
	inv := &kollectdevv1alpha1.KollectInventory{
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &interval,
		},
	}

	if got := rec.exportDebounce(inv); got != 5*time.Second {
		t.Fatalf("exportDebounce() = %v, want 5s from spec", got)
	}
}

func TestKollectInventoryReconciler_exportDebounce_crdDefault(t *testing.T) {
	t.Parallel()

	rec := &KollectInventoryReconciler{}
	inv := &kollectdevv1alpha1.KollectInventory{}

	if got := rec.exportDebounce(inv); got != 30*time.Second {
		t.Fatalf("exportDebounce() = %v, want CRD default 30s", got)
	}
}

// newPreviewDebounceReconciler builds a reconciler suitable for exercising
// previewAllSinksDebounced (WB-02) without any sink objects: loadResolvedSink
// failures fall back to the default interval, which is all the preview needs.
func newPreviewDebounceReconciler(t *testing.T) *KollectInventoryReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	return &KollectInventoryReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
}

// WB-02: when every sink is debounced, reconcile short-circuits before
// marshalling/exporting; these tests lock the short-circuit decision.
func TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced(t *testing.T) {
	t.Parallel()

	rec := newPreviewDebounceReconciler(t)
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg-a", "pg-b"),
		},
	}

	invKey := "default/team-inventory"
	checksum := "fingerprint-a"
	now := time.Now()
	bindings := inventorySinkBindings(inv)
	for _, binding := range bindings {
		rec.sinkCoalesce.record(invKey, sinkExportKey(binding), inv.Generation, checksum, now)
	}

	outcome, allDebounced := rec.previewAllSinksDebounced(inv, invKey, checksum)
	if !allDebounced {
		t.Fatal("previewAllSinksDebounced = false, want true when every sink is debounced")
	}
	if outcome.DebouncedCount != len(bindings) {
		t.Fatalf("DebouncedCount = %d, want %d", outcome.DebouncedCount, len(bindings))
	}
}

func TestKollectInventoryReconciler_previewAllSinksDebounced_oneSinkDue(t *testing.T) {
	t.Parallel()

	rec := newPreviewDebounceReconciler(t)
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg-a", "pg-b"),
		},
	}

	invKey := "default/team-inventory"
	checksum := "fingerprint-a"
	bindings := inventorySinkBindings(inv)
	// Only the first sink has a fresh export recorded; the second is due.
	rec.sinkCoalesce.record(invKey, sinkExportKey(bindings[0]), inv.Generation, checksum, time.Now())

	if _, allDebounced := rec.previewAllSinksDebounced(inv, invKey, checksum); allDebounced {
		t.Fatal("previewAllSinksDebounced = true, want false when a sink is due for export")
	}
}

func TestKollectInventoryReconciler_previewAllSinksDebounced_zeroBindings(t *testing.T) {
	t.Parallel()

	rec := newPreviewDebounceReconciler(t)
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
	}

	if _, allDebounced := rec.previewAllSinksDebounced(inv, "default/team-inventory", "fingerprint-a"); allDebounced {
		t.Fatal("previewAllSinksDebounced = true, want false with zero bindings")
	}
}
