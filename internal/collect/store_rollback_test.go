// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"
)

// EC-P0-03: hub ingest rolls back store state on export failure via CloneTargetItems + RestoreTarget.
func TestStoreCloneAndRestore_rollbackPriorBucket(t *testing.T) {
	t.Parallel()

	s := NewStore()
	item := Item{
		TargetNamespace: "hub",
		TargetName:      "spoke-a",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	}
	s.Upsert(item)

	prior := s.CloneTargetItems("hub", "spoke-a")
	if len(prior) != 1 || prior["uid-1"].Name != "web" {
		t.Fatalf("prior = %#v", prior)
	}

	s.Upsert(Item{
		TargetNamespace: "hub",
		TargetName:      "spoke-a",
		UID:             "uid-2",
		Namespace:       "apps",
		Name:            "api",
		Version:         "v1",
		Kind:            "Deployment",
	})
	if s.CountForTarget("hub", "spoke-a") != 2 {
		t.Fatalf("count after merge = %d", s.CountForTarget("hub", "spoke-a"))
	}

	s.RestoreTarget("hub", "spoke-a", prior)
	if got := s.CountForTarget("hub", "spoke-a"); got != 1 {
		t.Fatalf("count after rollback = %d", got)
	}
	snap := s.SnapshotTarget("hub", "spoke-a")
	if len(snap) != 1 || snap[0].UID != "uid-1" {
		t.Fatalf("snapshot after rollback = %#v", snap)
	}
}

func TestStoreCloneTargetItems_emptyReturnsNil(t *testing.T) {
	t.Parallel()

	s := NewStore()
	if got := s.CloneTargetItems("missing", "target"); got != nil {
		t.Fatalf("CloneTargetItems = %#v, want nil", got)
	}
}

func TestStoreRestoreTarget_nilPriorRemovesTarget(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "hub",
		TargetName:      "spoke-a",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})

	s.RestoreTarget("hub", "spoke-a", nil)
	if s.CountForTarget("hub", "spoke-a") != 0 {
		t.Fatalf("count after nil restore = %d", s.CountForTarget("hub", "spoke-a"))
	}
}
