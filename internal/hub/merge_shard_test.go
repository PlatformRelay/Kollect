// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/konih/kollect/internal/collect"
)

// TestMergerApply100Spokes simulates hub shard fan-in from 100 clusters with summarized
// payloads (10 items per spoke). Merge is O(total rows): ~1000 upserts here.
//
// Memory (linux/amd64, go test -count=1, 2026-06): heap rises ~300–500 KiB for 1000 minimal
// Item structs in the store; linear with row count. At 10k objects/spoke × 100 spokes the
// hub must shard consumers and avoid full snapshots (see ADR-0603).
func TestMergerApply100Spokes(t *testing.T) {
	t.Parallel()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	store := collect.NewStore()
	merger := NewMerger(store)

	const (
		spokeCount = 100
		itemsEach  = 10
	)

	for s := range spokeCount {
		cluster := fmt.Sprintf("spoke-%03d", s)
		report := SpokeReport{
			APIVersion: reportAPIVersion,
			Cluster:    cluster,
			InventoryRef: InventoryRef{
				Namespace: "platform",
				Name:      "rollup",
			},
			Items: summarizedItems(cluster, itemsEach),
		}

		if _, err := merger.Apply(report); err != nil {
			t.Fatalf("spoke %d: %v", s, err)
		}
	}

	wantTotal := spokeCount * itemsEach
	if got := store.TotalCount(); got != wantTotal {
		t.Fatalf("total = %d, want %d", got, wantTotal)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("heap alloc delta ~%d KiB for %d rows", (after.HeapAlloc-before.HeapAlloc)/1024, wantTotal)
}

func BenchmarkMerger100Spokes(b *testing.B) {
	store := collect.NewStore()
	merger := NewMerger(store)

	reports := make([]SpokeReport, 100)
	for i := range reports {
		cluster := fmt.Sprintf("spoke-%03d", i)
		reports[i] = SpokeReport{
			APIVersion: reportAPIVersion,
			Cluster:    cluster,
			InventoryRef: InventoryRef{
				Namespace: "platform",
				Name:      "rollup",
			},
			Items: summarizedItems(cluster, 10),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, report := range reports {
			if _, err := merger.Apply(report); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func summarizedItems(cluster string, n int) []collect.Item {
	items := make([]collect.Item, n)
	for i := range n {
		items[i] = collect.Item{
			Namespace: "apps",
			Name:      fmt.Sprintf("%s-app-%d", cluster, i),
			UID:       fmt.Sprintf("%s-uid-%d", cluster, i),
			Version:   "v1",
			Kind:      "Deployment",
			Attributes: map[string]any{
				"replicas": 1,
				"cluster":  cluster,
			},
		}
	}

	return items
}
