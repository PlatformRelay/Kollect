// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package aggregate

import (
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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

	if !c.ShouldSkip(now.Add(interval-time.Second), interval, 1, payloadA) {
		t.Fatal("still within interval should skip")
	}

	if c.ShouldSkip(now.Add(interval+time.Second), interval, 1, payloadA) {
		t.Fatal("after interval elapsed export should run again")
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

func TestDedupeModeFromSpec(t *testing.T) {
	t.Parallel()

	if got := DedupeModeFromSpec(nil); got != DedupeKeepAll {
		t.Fatalf("nil spec = %v, want DedupeKeepAll", got)
	}

	keepAll := DedupeModeFromSpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		Dedupe: kollectdevv1alpha1.ClusterInventoryDedupeKeepAll,
	})
	if keepAll != DedupeKeepAll {
		t.Fatalf("keepAll = %v", keepAll)
	}

	byUID := DedupeModeFromSpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		Dedupe: kollectdevv1alpha1.ClusterInventoryDedupeByResourceUID,
	})
	if byUID != DedupeByResourceUID {
		t.Fatalf("byResourceUID = %v", byUID)
	}

	defaultMode := DedupeModeFromSpec(&kollectdevv1alpha1.KollectClusterInventorySpec{})
	if defaultMode != DedupeKeepAll {
		t.Fatalf("empty dedupe = %v, want DedupeKeepAll", defaultMode)
	}
}

func FuzzContentHash(f *testing.F) {
	f.Add([]byte(`{"items":1}`))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0xff, 0xfe})

	f.Fuzz(func(t *testing.T, data []byte) {
		hash := ContentHash(data)
		if len(hash) != 64 {
			t.Fatalf("ContentHash length = %d, want 64 (sha256 hex)", len(hash))
		}

		for _, c := range hash {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				t.Fatalf("ContentHash %q contains non-hex character %q", hash, c)
			}
		}

		if got := ContentHash(data); got != hash {
			t.Fatalf("ContentHash not stable: %q vs %q", got, hash)
		}
	})
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

func TestExportCoalesceNilReceiver(t *testing.T) {
	t.Parallel()

	var coalesce *ExportCoalesce
	if coalesce.ShouldSkip(time.Now(), time.Minute, 1, []byte("x")) {
		t.Fatal("nil coalesce should not skip")
	}

	coalesce.RecordExport(time.Now(), 1, []byte("x"))
}

func TestResourceKeyFromItem(t *testing.T) {
	t.Parallel()

	key := ResourceKeyFromItem(collect.Item{Namespace: "apps", UID: "uid-1"})
	if key.Namespace != "apps" || key.UID != "uid-1" {
		t.Fatalf("key = %#v", key)
	}
}
