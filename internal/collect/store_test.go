// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"
)

func TestStoreNamespaceSnapshot(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		UID:             "1",
		Namespace:       "team-a",
		Name:            "app",
		Version:         "v1",
		Kind:            "Deployment",
		Attributes:      map[string]any{"name": "app"},
	})

	if got := s.CountForNamespace("team-a"); got != 1 {
		t.Fatalf("CountForNamespace = %d, want 1", got)
	}

	items := s.SnapshotNamespace("team-a")
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}

	s.RemoveTarget("team-a", "deploys")
	if got := s.CountForNamespace("team-a"); got != 0 {
		t.Fatalf("after remove count = %d", got)
	}
}

func TestStoreNamespaceIsolation(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "tenant-a",
		TargetName:      "deploys",
		UID:             "a1",
		Namespace:       "tenant-a",
		Name:            "app-a",
		Version:         "v1",
		Kind:            "Deployment",
	})
	s.Upsert(Item{
		TargetNamespace: "tenant-b",
		TargetName:      "deploys",
		UID:             "b1",
		Namespace:       "tenant-b",
		Name:            "app-b",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if got := s.CountForNamespace("tenant-a"); got != 1 {
		t.Fatalf("tenant-a count = %d, want 1", got)
	}
	if got := s.CountForNamespace("tenant-b"); got != 1 {
		t.Fatalf("tenant-b count = %d, want 1", got)
	}

	snapA := s.SnapshotNamespace("tenant-a")
	if len(snapA) != 1 || snapA[0].Name != "app-a" {
		t.Fatalf("tenant-a snapshot = %#v", snapA)
	}
}
