// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCollectInventorySinkBindings(t *testing.T) {
	t.Parallel()

	spec := &KollectInventorySpec{
		SnapshotSinkRefs: InventorySinkRefList{{Name: "git-a"}},
		DatabaseSinkRefs: InventorySinkRefList{{Name: "pg-b"}},
		EventSinkRefs:    InventorySinkRefList{{Name: "kafka-c"}},
	}
	bindings := CollectInventorySinkBindings(spec)
	if len(bindings) != 3 {
		t.Fatalf("bindings = %d", len(bindings))
	}
	if bindings[0].Family != SinkFamilySnapshot || bindings[0].Name != "git-a" {
		t.Fatalf("snapshot binding: %+v", bindings[0])
	}
	if bindings[1].Family != SinkFamilyDatabase {
		t.Fatalf("database family: %s", bindings[1].Family)
	}
	if bindings[2].Family != SinkFamilyEvent {
		t.Fatalf("event family: %s", bindings[2].Family)
	}
}

func TestCollectClusterInventorySinkBindings(t *testing.T) {
	t.Parallel()

	spec := &KollectClusterInventorySpec{
		DatabaseSinkRefs: InventorySinkRefList{{Name: "warehouse"}},
	}
	bindings := CollectClusterInventorySinkBindings(spec)
	if len(bindings) != 1 || bindings[0].Family != SinkFamilyDatabase {
		t.Fatalf("bindings: %+v", bindings)
	}
}

func TestTotalInventorySinkRefCount_nilSpec(t *testing.T) {
	t.Parallel()

	if TotalInventorySinkRefCount(nil) != 0 {
		t.Fatal("expected zero for nil spec")
	}
}

func TestTotalClusterInventorySinkRefCount(t *testing.T) {
	t.Parallel()

	spec := &KollectClusterInventorySpec{
		SnapshotSinkRefs: InventorySinkRefList{{Name: "a"}, {Name: "b"}},
		EventSinkRefs:    InventorySinkRefList{{Name: "c"}},
	}
	if got := TotalClusterInventorySinkRefCount(spec); got != 3 {
		t.Fatalf("count = %d", got)
	}
	if !ClusterInventoryUsesClusterSinks(spec) {
		t.Fatal("expected cluster sinks in use")
	}
}

func TestAllInventorySinkRefLists(t *testing.T) {
	t.Parallel()

	d := metav1.Duration{Duration: 0}
	spec := &KollectInventorySpec{
		DatabaseSinkRefs: InventorySinkRefList{{Name: "pg", ExportMinInterval: &d}},
	}
	lists := AllInventorySinkRefLists(spec)
	if len(lists) != 3 || len(lists[1]) != 1 {
		t.Fatalf("lists: %+v", lists)
	}
}

func TestConnectionTestSinkRefFamily(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ref    ConnectionTestSinkRef
		family string
		name   string
		ok     bool
	}{
		{ConnectionTestSinkRef{SnapshotSinkRef: "git"}, SinkFamilySnapshot, "git", true},
		{ConnectionTestSinkRef{DatabaseSinkRef: "pg"}, SinkFamilyDatabase, "pg", true},
		{ConnectionTestSinkRef{EventSinkRef: "nats"}, SinkFamilyEvent, "nats", true},
		{ConnectionTestSinkRef{}, "", "", false},
		{ConnectionTestSinkRef{SnapshotSinkRef: "a", DatabaseSinkRef: "b"}, SinkFamilyDatabase, "b", false},
	}
	for _, tc := range cases {
		family, name, ok := tc.ref.Family()
		if family != tc.family || name != tc.name || ok != tc.ok {
			t.Fatalf("ref %+v => family=%q name=%q ok=%v", tc.ref, family, name, ok)
		}
	}
}
