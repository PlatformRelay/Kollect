// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func TestFamilySinkKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		family        string
		clusterScoped bool
		want          string
	}{
		{name: "snapshot namespaced", family: kollectdevv1alpha1.SinkFamilySnapshot, want: "KollectSnapshotSink"},
		{name: "database namespaced", family: kollectdevv1alpha1.SinkFamilyDatabase, want: "KollectDatabaseSink"},
		{name: "event namespaced", family: kollectdevv1alpha1.SinkFamilyEvent, want: "KollectEventSink"},
		{name: "unknown namespaced", family: "custom", want: "KollectSink"},
		{name: "snapshot cluster", family: kollectdevv1alpha1.SinkFamilySnapshot, clusterScoped: true, want: "KollectClusterSnapshotSink"},
		{name: "database cluster", family: kollectdevv1alpha1.SinkFamilyDatabase, clusterScoped: true, want: "KollectClusterDatabaseSink"},
		{name: "event cluster", family: kollectdevv1alpha1.SinkFamilyEvent, clusterScoped: true, want: "KollectClusterEventSink"},
		{name: "unknown cluster", family: "custom", clusterScoped: true, want: "KollectClusterSink"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := familySinkKind(tc.family, tc.clusterScoped); got != tc.want {
				t.Fatalf("familySinkKind(%q, %t) = %q, want %q", tc.family, tc.clusterScoped, got, tc.want)
			}
		})
	}
}

func TestInventoryBindingHelpers_NilSafe(t *testing.T) {
	t.Parallel()

	if got := inventorySinkBindings(nil); got != nil {
		t.Fatalf("inventorySinkBindings(nil) = %#v, want nil", got)
	}
	if got := clusterInventorySinkBindings(nil); got != nil {
		t.Fatalf("clusterInventorySinkBindings(nil) = %#v, want nil", got)
	}
}

func TestTotalSinkRefHelpers(t *testing.T) {
	t.Parallel()

	inv := &kollectdevv1alpha1.KollectInventory{
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("snap-a"),
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("db-a", "db-b"),
			EventSinkRefs:    kollectdevv1alpha1.NewSinkRefList("events-a"),
		},
	}
	clusterInv := &kollectdevv1alpha1.KollectClusterInventory{
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("csnap-a"),
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("cdb-a"),
			EventSinkRefs:    kollectdevv1alpha1.NewSinkRefList("cfam-a"),
		},
	}

	if got := totalInventorySinkRefs(inv); got != 4 {
		t.Fatalf("totalInventorySinkRefs = %d, want 4", got)
	}
	if got := totalClusterInventorySinkRefs(clusterInv); got != 3 {
		t.Fatalf("totalClusterInventorySinkRefs = %d, want 3", got)
	}
}

func TestSinkExportHelpers(t *testing.T) {
	t.Parallel()

	if got := sinkExportKey(kollectdevv1alpha1.InventorySinkBinding{Family: "database", Name: "primary"}); got != "database/primary" {
		t.Fatalf("sinkExportKey = %q, want database/primary", got)
	}

	if got := sinkExportMinInterval(nil); got != nil {
		t.Fatalf("sinkExportMinInterval(nil) = %#v, want nil", got)
	}

	if got := sinkExportMinInterval(&sink.ResolvedSink{}); got != nil {
		t.Fatalf("sinkExportMinInterval(empty) = %#v, want nil", got)
	}

	d := metav1.Duration{Duration: 30 * time.Second}
	resolved := &sink.ResolvedSink{
		ExportMinInterval: &kollectdevv1alpha1.SinkCommonFields{ExportMinInterval: &d},
	}
	got := sinkExportMinInterval(resolved)
	if got == nil || got.Duration != 30*time.Second {
		t.Fatalf("sinkExportMinInterval(resolved) = %#v, want 30s", got)
	}
}
