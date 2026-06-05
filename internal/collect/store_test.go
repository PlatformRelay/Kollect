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
