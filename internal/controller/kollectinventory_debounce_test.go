// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectInventoryReconciler_shouldDebounce(t *testing.T) {
	t.Parallel()

	rec := &KollectInventoryReconciler{
		Options: RuntimeOptions{ExportDebounce: 30 * time.Second},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "team-inventory", Generation: 1},
	}

	hashA := "fingerprint-a"
	hashB := "fingerprint-b"
	key := "default/team-inventory"

	if rec.shouldDebounce(inv, key, hashA) {
		t.Fatal("first export must not debounce")
	}

	rec.recordExport(inv, key, hashA)

	if !rec.shouldDebounce(inv, key, hashA) {
		t.Fatal("identical payload within interval should debounce")
	}

	if rec.shouldDebounce(inv, key, hashB) {
		t.Fatal("payload change must bypass debounce")
	}

	inv.Generation = 2
	if rec.shouldDebounce(inv, key, hashA) {
		t.Fatal("spec generation bump must bypass debounce")
	}
}

func TestKollectInventoryReconciler_exportDebounce_perInventory(t *testing.T) {
	t.Parallel()

	interval := metav1.Duration{Duration: 5 * time.Second}
	rec := &KollectInventoryReconciler{
		Options: RuntimeOptions{ExportDebounce: 30 * time.Second},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &interval,
		},
	}

	if got := rec.exportDebounce(inv); got != 5*time.Second {
		t.Fatalf("exportDebounce() = %v, want 5s from spec", got)
	}
}

func TestKollectInventoryReconciler_exportDebounce_fallback(t *testing.T) {
	t.Parallel()

	rec := &KollectInventoryReconciler{
		Options: RuntimeOptions{ExportDebounce: 12 * time.Second},
	}
	inv := &kollectdevv1alpha1.KollectInventory{}

	if got := rec.exportDebounce(inv); got != 12*time.Second {
		t.Fatalf("exportDebounce() = %v, want global fallback 12s", got)
	}
}
