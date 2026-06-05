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

func TestStoreSubscribeAndMarshal(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	s.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})

	select {
	case <-ch:
	default:
		t.Fatal("expected watcher notification")
	}

	if got := s.CountForTarget("team-a", "deploys"); got != 1 {
		t.Fatalf("CountForTarget = %d", got)
	}

	payload, err := s.MarshalTargetJSON("team-a", "deploys")
	if err != nil || len(payload) == 0 {
		t.Fatalf("MarshalTargetJSON: %v (%q)", err, payload)
	}

	nsPayload, err := s.MarshalNamespaceJSON("team-a")
	if err != nil || len(nsPayload) == 0 {
		t.Fatalf("MarshalNamespaceJSON: %v", err)
	}

	s.Remove("team-a", "deploys", "uid-1")
	if s.TotalCount() != 0 {
		t.Fatalf("TotalCount = %d", s.TotalCount())
	}

	summary := s.Summary("")
	if summary.ItemCount != 0 {
		t.Fatalf("empty summary count = %d", summary.ItemCount)
	}
}

func TestStoreLenAndNamespaceExport(t *testing.T) {
	t.Parallel()

	s := NewStore()
	if s.Len() != 0 {
		t.Fatalf("Len = %d", s.Len())
	}

	s.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "inv",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if s.Len() != 1 {
		t.Fatalf("Len = %d", s.Len())
	}

	snap := s.SnapshotTarget("team-a", "inv")
	if len(snap) != 1 {
		t.Fatalf("snapshot = %d", len(snap))
	}

	payload, err := s.MarshalNamespaceExport("team-a", ExportMetadata{Cluster: "spoke-a"})
	if err != nil || len(payload) == 0 {
		t.Fatalf("MarshalNamespaceExport: %v", err)
	}
}

func TestStoreSummaryAllNamespaces(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})
	s.Upsert(Item{
		TargetNamespace: "team-b",
		TargetName:      "pods",
		UID:             "uid-2",
		Namespace:       "apps",
		Name:            "api",
		Version:         "v1",
		Kind:            "Pod",
	})

	all := s.Summary("")
	if all.ItemCount != 2 {
		t.Fatalf("all summary count = %d", all.ItemCount)
	}

	emptyTarget := s.SnapshotTarget("missing", "target")
	if emptyTarget != nil {
		t.Fatalf("empty target snapshot = %#v", emptyTarget)
	}

	s.Remove("team-a", "deploys", "uid-1")
	if s.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("count after remove uid = %d", s.CountForTarget("team-a", "deploys"))
	}
}
