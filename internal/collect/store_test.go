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

// TestStoreNamespaceVersion_BumpsOnMutationAndIsolatesNamespaces backs AR-10
// (PERF-01 remainder): the version counter is the cheap "did anything change"
// signal a reconciler can check before paying for a full SnapshotNamespace +
// content fingerprint.
func TestStoreNamespaceVersion_BumpsOnMutationAndIsolatesNamespaces(t *testing.T) {
	t.Parallel()

	s := NewStore()

	v0 := s.NamespaceVersion("ns-a")

	s.Upsert(Item{
		TargetNamespace: "ns-a",
		TargetName:      "deploys",
		UID:             "1",
		Namespace:       "ns-a",
		Name:            "app",
		Version:         "v1",
		Kind:            "Deployment",
	})
	v1 := s.NamespaceVersion("ns-a")
	if v1 == v0 {
		t.Fatalf("NamespaceVersion did not change after Upsert: v0=%d v1=%d", v0, v1)
	}

	// Mutating an unrelated namespace must not bump ns-a's version.
	s.Upsert(Item{
		TargetNamespace: "ns-b",
		TargetName:      "deploys",
		UID:             "1",
		Namespace:       "ns-b",
		Name:            "app",
		Version:         "v1",
		Kind:            "Deployment",
	})
	if got := s.NamespaceVersion("ns-a"); got != v1 {
		t.Fatalf("ns-a version changed after unrelated ns-b mutation: got %d, want %d", got, v1)
	}

	// A second Upsert (even of the same item) bumps the version again — the
	// counter signals "a mutation happened", not "content differs"; content
	// comparison stays the job of the real fingerprint.
	s.Upsert(Item{
		TargetNamespace: "ns-a",
		TargetName:      "deploys",
		UID:             "1",
		Namespace:       "ns-a",
		Name:            "app",
		Version:         "v1",
		Kind:            "Deployment",
	})
	v2 := s.NamespaceVersion("ns-a")
	if v2 == v1 {
		t.Fatalf("NamespaceVersion did not change after second Upsert: v1=%d v2=%d", v1, v2)
	}

	s.Remove("ns-a", "deploys", "1")
	v3 := s.NamespaceVersion("ns-a")
	if v3 == v2 {
		t.Fatalf("NamespaceVersion did not change after Remove: v2=%d v3=%d", v2, v3)
	}
}

// TestStoreNamespaceVersion_MonotonicAcrossRemoveClusterAndShardRecreation
// guards a specific trap: RemoveCluster deletes the whole storeShard for a
// namespace, so if the version counter lived on the shard itself, the next
// Upsert into that namespace would recreate the shard at version 0 and could
// climb back to a version number a stale cache entry was holding — a true
// (version, content) collision, not just a missed cache hit. The version
// must survive shard deletion and never repeat a previously-issued value for
// the same namespace.
func TestStoreNamespaceVersion_MonotonicAcrossRemoveClusterAndShardRecreation(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.Upsert(Item{
		TargetNamespace: "ns-a",
		TargetName:      "deploys",
		UID:             "1",
		Namespace:       "ns-a",
		Name:            "app",
		Version:         "v1",
		Kind:            "Deployment",
	})
	vBefore := s.NamespaceVersion("ns-a")

	s.RemoveCluster("ns-a")
	vAfterRemove := s.NamespaceVersion("ns-a")
	if vAfterRemove <= vBefore {
		t.Fatalf("version must strictly increase after RemoveCluster, not reset: before=%d after=%d", vBefore, vAfterRemove)
	}

	// Repopulate the namespace (shard recreated from scratch) enough times
	// that a naive shard-local counter reset to 0 would climb back through
	// vBefore and vAfterRemove (both are small single-digit values here).
	var vAfterRepopulate uint64
	for i := range 5 {
		s.Upsert(Item{
			TargetNamespace: "ns-a",
			TargetName:      "deploys",
			UID:             "1",
			Namespace:       "ns-a",
			Name:            "app",
			Version:         "v2",
			Kind:            "Deployment",
			Attributes:      map[string]any{"i": i},
		})
		v := s.NamespaceVersion("ns-a")
		if v == vBefore {
			t.Fatalf("version repeated a previously-issued value (vBefore=%d) after RemoveCluster + repopulate — a cache entry recorded at vBefore would now incorrectly hit", vBefore)
		}
		if v == vAfterRemove {
			t.Fatalf("version repeated a previously-issued value (vAfterRemove=%d) after repopulate — a cache entry recorded at vAfterRemove would now incorrectly hit", vAfterRemove)
		}
		vAfterRepopulate = v
	}

	if vAfterRepopulate <= vAfterRemove {
		t.Fatalf("version must keep increasing across repopulation: vAfterRemove=%d vAfterRepopulate=%d", vAfterRemove, vAfterRepopulate)
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

func TestStoreSubscribeNamespaces(t *testing.T) {
	t.Parallel()

	s := NewStore()
	ch := s.SubscribeNamespaces()
	defer s.UnsubscribeNamespaces(ch)

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
	case ns := <-ch:
		if ns != "team-a" {
			t.Fatalf("namespace = %q, want team-a", ns)
		}
	default:
		t.Fatal("expected namespace watcher notification")
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
