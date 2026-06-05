// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"testing"

	"github.com/konih/kollect/internal/collect"
)

func TestMergerApply(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := NewMerger(store)

	report := SpokeReport{
		APIVersion: reportAPIVersion,
		Cluster:    "prod-eu-1",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "team-inventory",
		},
		Items: []collect.Item{
			{
				Namespace: "apps",
				Name:      "demo",
				UID:       "uid-1",
				Version:   "v1",
				Kind:      "Deployment",
			},
			{
				Namespace: "apps",
				Name:      "demo-2",
				UID:       "uid-2",
				Version:   "v1",
				Kind:      "Deployment",
			},
		},
	}

	applied, err := merger.Apply(report)
	if err != nil {
		t.Fatal(err)
	}

	if applied != 2 {
		t.Fatalf("applied = %d, want 2", applied)
	}

	if store.TotalCount() != 2 {
		t.Fatalf("store count = %d, want 2", store.TotalCount())
	}

	if store.CountForTarget("prod-eu-1", "team-inventory") != 2 {
		t.Fatalf("target count = %d, want 2", store.CountForTarget("prod-eu-1", "team-inventory"))
	}

	// Re-apply is idempotent (same UIDs, no duplicate rows).
	if _, err := merger.Apply(report); err != nil {
		t.Fatal(err)
	}

	if store.TotalCount() != 2 {
		t.Fatalf("after re-apply store count = %d, want 2", store.TotalCount())
	}
}

func TestMergerApplyJSON(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := NewMerger(store)

	payload := []byte(`{
		"cluster":"spoke-a",
		"inventoryRef":{"namespace":"default","name":"inv"},
		"items":[{"namespace":"ns","name":"n","uid":"u1","version":"v1","kind":"Pod"}]
	}`)

	applied, err := merger.ApplyJSON(payload)
	if err != nil {
		t.Fatal(err)
	}

	if applied != 1 {
		t.Fatalf("applied = %d, want 1", applied)
	}
}

func TestMergerApplyRequiresCluster(t *testing.T) {
	t.Parallel()

	merger := NewMerger(collect.NewStore())
	if _, err := merger.Apply(SpokeReport{}); err == nil {
		t.Fatal("expected error for empty cluster")
	}
}
