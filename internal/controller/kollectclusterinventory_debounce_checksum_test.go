// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/metrics"
	"github.com/platformrelay/kollect/internal/sink"
)

// TestClusterInventory_MultipartExport_DebounceUsesPartitionsChecksum is the multipart-digest differential
// lock: the cluster path must debounce and record on the multipart digest for snapshot-family
// bindings, exactly like the namespaced path (kollectinventory_controller.go:352-355). Before
// the fix it recorded the raw content checksum, so a global --max-export-bytes change (which
// alters part boundaries but not content) was silently ignored.
func TestClusterInventory_MultipartExport_DebounceUsesPartitionsChecksum(t *testing.T) {
	t.Parallel()

	const (
		sinkNS = sink.DefaultSecretNamespace
		target = "platform-deployments"
	)

	items := make([]collect.Item, 0, 3)
	for i := range 3 {
		items = append(items, collect.Item{
			TargetNamespace: "tenant-a",
			TargetName:      target,
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       "tenant-a",
			Name:            fmt.Sprintf("app-%d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"payload": strings.Repeat("x", 220)},
		})
	}

	scheme := clusterRollupScheme(t)

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"},
		},
	}

	limit := int64(900)
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "git-platform", MaxExportBytes: &limit}},
			SinkNamespace:    sinkNS,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Registry: reg,
		Recorder: record.NewFakeRecorder(10),
	}

	const rawChecksum = "raw-content-checksum"
	outcome := rec.exportClusterToSinks(
		context.Background(), logr.Discard(), inv, "cluster/platform-rollup", sinkNS, items, rawChecksum,
	)
	if outcome.ExportedCount != 1 || len(outcome.SinkExports) != 1 {
		t.Fatalf("outcome = %+v, want one exported sink", outcome)
	}

	parts, err := export.PartitionEnvelopes(items, export.Metadata{Generation: inv.Generation}, limit)
	if err != nil {
		t.Fatalf("PartitionEnvelopes: %v", err)
	}
	if len(parts) < 2 {
		t.Fatalf("parts = %d, want >= 2 (test did not force multipart)", len(parts))
	}
	want := export.PartitionsChecksum(parts)

	got := outcome.SinkExports[0].LastChecksum
	if got == rawChecksum {
		t.Fatal("cluster debounce recorded the raw content checksum; multipart-digest debounce regression")
	}
	if got != want {
		t.Fatalf("LastChecksum = %q, want multipart digest %q", got, want)
	}
}

// TestClusterInventory_DebouncedExportIncrementsMetric is the debounced-metric lock: a debounced cluster
// export must increment kollect_export_debounced_total for KollectClusterInventory, so cluster
// debounce decisions are visible to dashboards/alerts exactly like the namespaced path.
func TestClusterInventory_DebouncedExportIncrementsMetric(t *testing.T) {
	t.Parallel()

	const (
		sinkNS = sink.DefaultSecretNamespace
		target = "platform-deployments"
	)

	items := []collect.Item{{
		TargetNamespace: "tenant-a", TargetName: target, UID: "uid-0",
		Namespace: "tenant-a", Name: "app", Version: "v1", Kind: "Deployment",
		Attributes: map[string]any{"image": "nginx:1.27"},
	}}

	scheme := clusterRollupScheme(t)

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"},
		},
	}

	longInterval := metav1.Duration{Duration: 5 * time.Minute}
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			ExportMinInterval: &longInterval,
			SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("git-platform"),
			SinkNamespace:     sinkNS,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Registry: reg,
		Recorder: record.NewFakeRecorder(10),
	}

	const checksum = "stable-checksum"
	invKey := "cluster/platform-rollup"

	first := rec.exportClusterToSinks(context.Background(), logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}

	before := counterValue(metrics.ExportDebouncedTotal, "KollectClusterInventory")
	second := rec.exportClusterToSinks(context.Background(), logr.Discard(), inv, invKey, sinkNS, items, checksum)
	after := counterValue(metrics.ExportDebouncedTotal, "KollectClusterInventory")

	if second.DebouncedCount != 1 || second.ExportedCount != 0 {
		t.Fatalf("second export = %d exported / %d debounced, want 0/1", second.ExportedCount, second.DebouncedCount)
	}
	if after-before != 1 {
		t.Fatalf("ExportDebouncedTotal{controller=KollectClusterInventory} delta = %v, want 1", after-before)
	}
}

// TestClusterInventory_NonSnapshotExport_KeepsRawChecksum pins the other half of the digest rule: database
// and event bindings are not partitioned, so they must keep the raw content checksum.
func TestClusterInventory_NonSnapshotExport_KeepsRawChecksum(t *testing.T) {
	t.Parallel()

	const (
		sinkNS = sink.DefaultSecretNamespace
		target = "platform-deployments"
	)

	items := []collect.Item{{
		TargetNamespace: "tenant-a", TargetName: target, UID: "uid-0",
		Namespace: "tenant-a", Name: "app", Version: "v1", Kind: "Deployment",
		Attributes: map[string]any{"image": "nginx:1.27"},
	}}

	scheme := clusterRollupScheme(t)

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type:     kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"}, Table: "inventory_items"},
		},
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace:    sinkNS,
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Registry: reg,
		Recorder: record.NewFakeRecorder(10),
	}

	const rawChecksum = "raw-content-checksum"
	outcome := rec.exportClusterToSinks(
		context.Background(), logr.Discard(), inv, "cluster/platform-rollup", sinkNS, items, rawChecksum,
	)
	if outcome.ExportedCount != 1 || len(outcome.SinkExports) != 1 {
		t.Fatalf("outcome = %+v, want one exported sink", outcome)
	}
	if got := outcome.SinkExports[0].LastChecksum; got != rawChecksum {
		t.Fatalf("non-snapshot LastChecksum = %q, want raw %q", got, rawChecksum)
	}
}
