// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
