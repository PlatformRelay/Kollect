// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import "testing"

func TestStoreRemoveCluster(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "spoke-a",
		TargetName:      "team-inventory",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "demo",
		Version:         "v1",
		Kind:            "Deployment",
	})
	s.Upsert(Item{
		TargetNamespace: "spoke-b",
		TargetName:      "team-inventory",
		UID:             "uid-2",
		Namespace:       "apps",
		Name:            "other",
		Version:         "v1",
		Kind:            "Deployment",
	})

	s.RemoveCluster("spoke-a")

	if got := s.SnapshotTarget("spoke-a", "team-inventory"); len(got) != 0 {
		t.Fatalf("spoke-a items = %d, want 0", len(got))
	}

	if got := s.SnapshotTarget("spoke-b", "team-inventory"); len(got) != 1 {
		t.Fatalf("spoke-b items = %d, want 1", len(got))
	}
}
