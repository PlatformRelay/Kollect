// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink"
)

// sizeBandBackend records the payloads a sink receives. Its capabilities decide
// whether it is a snapshot (multipart-capable, non-spill) or relational store.
type sizeBandBackend struct {
	caps     sink.Capabilities
	mu       sync.Mutex
	payloads [][]byte
	paths    []string
}

func (b *sizeBandBackend) Type() string { return "size-band" }

func (b *sizeBandBackend) Capabilities() sink.Capabilities { return b.caps }

func (b *sizeBandBackend) Export(_ context.Context, payload []byte, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.payloads = append(b.payloads, append([]byte(nil), payload...))
	b.paths = append(b.paths, path)

	return nil
}

func (b *sizeBandBackend) exportedCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.payloads)
}

// sizeBandItems builds n items whose combined JSON envelope lands in the given
// size band. Each item carries blobBytes of filler so the envelope clears the
// intended threshold.
func sizeBandItems(n, blobBytes int) []collect.Item {
	items := make([]collect.Item, 0, n)
	for i := range n {
		items = append(items, collect.Item{
			TargetNamespace: "default",
			TargetName:      "band-target",
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       "default",
			Name:            fmt.Sprintf("obj-%d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"blob": strings.Repeat("x", blobBytes)},
		})
	}

	return items
}

func sizeBandScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	return scheme
}

func gitSnapshotSink(name string) *kollectdevv1alpha1.KollectSnapshotSink {
	return &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "https://example.com/inventory.git",
			},
		},
	}
}

func postgresSink(name string) *kollectdevv1alpha1.KollectDatabaseSink {
	return &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		},
	}
}

// TestExportSizeBand_gitOnlyOversizeDegrades is the K-01 end-to-end lock: a
// git-only binding whose payload is above the inline cap must report
// Degraded/SpillRequired and must never write to the sink or record Exported.
func TestExportSizeBand_gitOnlyOversizeDegrades(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	for _, item := range sizeBandItems(3, 400_000) {
		store.Upsert(item)
	}

	scheme := sizeBandScheme(t)
	sinkObj := gitSnapshotSink("git-oversize")
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "git-only", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-oversize"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	gitBackend := &sizeBandBackend{caps: sink.SnapshotStoreCapabilities()}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return gitBackend, nil
	})
	t.Cleanup(func() { sink.EvictBackendPool("default", "git-oversize") })

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "git-only", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if gitBackend.exportedCount() != 0 {
		t.Fatalf("git backend exported %d payload(s), want 0 (oversize must not silently drop OR write)",
			gitBackend.exportedCount())
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "git-only", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != spillReasonSpillRequired {
		t.Fatalf("Degraded condition = %+v, want True/SpillRequired", degraded)
	}

	ready := apimeta.FindStatusCondition(got.Status.Conditions, conditionReady)
	if ready != nil && ready.Status == metav1.ConditionTrue && ready.Reason == "Exported" {
		t.Fatalf("Ready condition = %+v, want not Exported on a spill-required export", ready)
	}

	// A total export failure routes through setInventoryDegraded, which persists
	// the aggregate condition; per-sink status persistence on total failure is a
	// separate finding (B8). Assert the authoritative aggregate signal and that
	// no per-sink status reads green when one is present.
	for _, st := range got.Status.SinkExports {
		synced := apimeta.FindStatusCondition(st.Conditions, conditionSinkSynced)
		if synced != nil && synced.Status == metav1.ConditionTrue {
			t.Fatalf("sink %q reported Synced=True (%s), want failure", st.Name, synced.Reason)
		}
	}
}

// TestExportSizeBand_gitPostgresMixedNotSilentlyGreen covers the mixed K-01
// lock: with a snapshot sink bound the pre-export gate is skipped, so the git
// sink must still fail loudly and the postgres sink must not read green.
func TestExportSizeBand_gitPostgresMixedNotSilentlyGreen(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	for _, item := range sizeBandItems(3, 400_000) {
		store.Upsert(item)
	}

	scheme := sizeBandScheme(t)
	gitSink := gitSnapshotSink("git-mixed")
	pgSink := postgresSink("pg-mixed")
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-mixed"),
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg-mixed"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gitSink, pgSink, inv).
		WithStatusSubresource(gitSink, pgSink, inv).
		Build()

	gitBackend := &sizeBandBackend{caps: sink.SnapshotStoreCapabilities()}
	pgBackend := &sizeBandBackend{caps: sink.RelationalStoreCapabilities()}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return gitBackend, nil
	})
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return pgBackend, nil
	})
	t.Cleanup(func() {
		sink.EvictBackendPool("default", "git-mixed")
		sink.EvictBackendPool("default", "pg-mixed")
	})

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mixed", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if gitBackend.exportedCount() != 0 || pgBackend.exportedCount() != 0 {
		t.Fatalf("backends exported git=%d pg=%d, want 0/0 above the inline cap",
			gitBackend.exportedCount(), pgBackend.exportedCount())
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "mixed", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != spillReasonSpillRequired {
		t.Fatalf("Degraded condition = %+v, want True/SpillRequired", degraded)
	}

	ready := apimeta.FindStatusCondition(got.Status.Conditions, conditionReady)
	if ready != nil && ready.Status == metav1.ConditionTrue && ready.Reason == "Exported" {
		t.Fatalf("Ready condition = %+v, want not Exported (git failed)", ready)
	}

	// Both sinks are non-object-store and the payload is above the inline cap, so
	// the mixed case is a total failure: no sink may read green. (Total-failure
	// per-sink persistence is B8; the aggregate Degraded/SpillRequired above is
	// the authoritative signal.)
	for _, st := range got.Status.SinkExports {
		synced := apimeta.FindStatusCondition(st.Conditions, conditionSinkSynced)
		if synced != nil && synced.Status == metav1.ConditionTrue {
			t.Fatalf("sink %q reported Synced=True, want failure (no silently-green sink)", st.Name)
		}
	}
}

// TestExportSizeBand_postgresCeilingKeepsAllRows is the K-02 end-to-end lock: a
// postgres binding with a per-binding ceiling below the snapshot size must
// receive one complete payload, not parts whose diff-delete erases each other.
func TestExportSizeBand_postgresCeilingKeepsAllRows(t *testing.T) {
	t.Parallel()

	const itemCount = 3
	store := collect.NewStore()
	for _, item := range sizeBandItems(itemCount, 240_000) {
		store.Upsert(item)
	}

	// The complete envelope is above the 500 KiB per-binding ceiling but below
	// the 1 MiB inline cap, so partitioning (old behaviour) would tear it while
	// the fix sends one complete part.
	ceiling := int64(500 * 1024)

	scheme := sizeBandScheme(t)
	pgSink := postgresSink("pg-ceiling")
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-only", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "pg-ceiling", MaxExportBytes: &ceiling},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pgSink, inv).
		WithStatusSubresource(pgSink, inv).
		Build()

	pgBackend := &sizeBandBackend{caps: sink.RelationalStoreCapabilities()}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return pgBackend, nil
	})
	t.Cleanup(func() { sink.EvictBackendPool("default", "pg-ceiling") })

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "pg-only", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if pgBackend.exportedCount() != 1 {
		t.Fatalf("postgres backend received %d payload(s), want 1 complete snapshot",
			pgBackend.exportedCount())
	}

	rows, err := collect.ItemsFromExportPayload(pgBackend.payloads[0])
	if err != nil {
		t.Fatalf("decode exported payload: %v", err)
	}
	if len(rows) != itemCount {
		t.Fatalf("exported rows = %d, want %d (all rows must survive)", len(rows), itemCount)
	}

	// Sanity: the fixture really did exceed the per-binding ceiling, otherwise
	// this test would not exercise the partition/delete path.
	envelope, err := export.MarshalEnvelope(store.SnapshotNamespace("default"), export.Metadata{Generation: 1})
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	if int64(len(envelope)) <= ceiling {
		t.Fatalf("fixture envelope %d bytes must exceed the %d-byte ceiling", len(envelope), ceiling)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "pg-only", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded); degraded != nil &&
		degraded.Status == metav1.ConditionTrue {
		t.Fatalf("Degraded condition = %+v, want no degradation for a complete postgres export", degraded)
	}
	ready := apimeta.FindStatusCondition(got.Status.Conditions, conditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != "Exported" {
		t.Fatalf("Ready condition = %+v, want True/Exported", ready)
	}
	if len(got.Status.SinkExports) != 1 {
		t.Fatalf("sink exports = %d, want 1", len(got.Status.SinkExports))
	}
	synced := apimeta.FindStatusCondition(got.Status.SinkExports[0].Conditions, conditionSinkSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue || synced.Reason != "Exported" {
		t.Fatalf("sink Synced = %+v, want True/Exported", synced)
	}
}
