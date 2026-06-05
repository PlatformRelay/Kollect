// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package aggregate

import (
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestContentHashStable(t *testing.T) {
	t.Parallel()

	a := ContentHash([]byte(`{"items":1}`))
	b := ContentHash([]byte(`{"items":1}`))
	if a != b {
		t.Fatalf("hash mismatch: %q vs %q", a, b)
	}

	c := ContentHash([]byte(`{"items":2}`))
	if a == c {
		t.Fatal("different payloads must produce different hashes")
	}
}

func TestExportCoalesceShouldSkip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	payloadA := []byte(`{"items":1}`)
	payloadB := []byte(`{"items":2}`)
	interval := 30 * time.Second

	var c ExportCoalesce
	if c.ShouldSkip(now, interval, 1, payloadA) {
		t.Fatal("first export must not skip")
	}

	c.RecordExport(now, 1, payloadA)

	if !c.ShouldSkip(now.Add(5*time.Second), interval, 1, payloadA) {
		t.Fatal("identical payload within interval should skip")
	}

	if c.ShouldSkip(now.Add(5*time.Second), interval, 1, payloadB) {
		t.Fatal("payload change must not skip")
	}

	if c.ShouldSkip(now.Add(5*time.Second), interval, 2, payloadA) {
		t.Fatal("generation bump must not skip")
	}
}

func TestMergeRowsByResourceUID(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{
			TargetNamespace: "team-a",
			TargetName:      "deployments",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 1},
		},
		{
			TargetNamespace: "team-a",
			TargetName:      "deployments-alt",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 3},
		},
	}

	keepAll := MergeRows(items, DedupeKeepAll)
	if len(keepAll) != 2 {
		t.Fatalf("DedupeKeepAll len = %d, want 2", len(keepAll))
	}

	byUID := MergeRows(items, DedupeByResourceUID)
	if len(byUID) != 1 {
		t.Fatalf("DedupeByResourceUID len = %d, want 1", len(byUID))
	}

	if got := byUID[0].Attributes["replicas"]; got != 3 {
		t.Fatalf("last row wins: replicas = %v, want 3", got)
	}
}

func TestIdentityFromItem(t *testing.T) {
	t.Parallel()

	item := collect.Item{
		TargetNamespace: "ns",
		TargetName:      "tgt",
		Namespace:       "apps",
		Name:            "demo",
		UID:             "uid-abc",
	}

	id := IdentityFromItem(item)
	if id.TargetNamespace != "ns" || id.TargetName != "tgt" || id.UID != "uid-abc" {
		t.Fatalf("IdentityFromItem() = %#v", id)
	}
}
