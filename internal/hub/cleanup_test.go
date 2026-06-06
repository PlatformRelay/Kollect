// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"testing"

	"github.com/konih/kollect/internal/collect"
)

func TestCleanupCluster(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "spoke-a",
		TargetName:      "inv",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "demo",
		Version:         "v1",
		Kind:            "Deployment",
	})

	CleanupCluster(store, "spoke-a")

	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0", store.TotalCount())
	}
}

func TestCleanupClusterNilStore(t *testing.T) {
	t.Parallel()

	CleanupCluster(nil, "spoke-a")
}
