// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
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

// partFailingBackend records every exported payload/path and fails the export
// whose 1-based call number equals failOnCall, simulating a torn multipart set
// where an earlier part is already persisted before a later part fails (REL-02).
type partFailingBackend struct {
	mu         sync.Mutex
	exported   [][]byte
	paths      []string
	calls      int
	failOnCall int
}

func (b *partFailingBackend) Type() string { return "part-failing" }

func (b *partFailingBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (b *partFailingBackend) Export(_ context.Context, payload []byte, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	if b.calls == b.failOnCall {
		return errors.New("simulated part export failure")
	}
	b.exported = append(b.exported, append([]byte(nil), payload...))
	b.paths = append(b.paths, path)

	return nil
}

// TestKollectInventoryReconciler_tornMultipartExportIsDetectable is the REL-02
// controller failure-mode test. A 3-part snapshot export fails on part 2; the
// already-persisted part 1 must carry the completeness marker (partIndex=1,
// partTotal=3 + generation) so a consumer can tell the set is torn, and the
// resource status must reflect the failure (Degraded), never success.
func TestKollectInventoryReconciler_tornMultipartExportIsDetectable(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	for i := range 3 {
		store.Upsert(collect.Item{
			TargetNamespace: "default",
			TargetName:      "nginx-deployments",
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       "default",
			Name:            fmt.Sprintf("nginx-%d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"payload": strings.Repeat("x", 220)},
		})
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	// A tight ceiling forces one item per part → three parts (a 1-item marked
	// envelope is ~627 bytes, a 2-item one ~1026).
	limit := int64(900)
	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "https://example.com/inventory.git",
				// Pin JSON so the raw envelope (with completeness markers) is
				// written unchanged instead of YAML-projected (ADR-0419).
				Serialization: &kollectdevv1alpha1.SerializationSpec{
					Format: kollectdevv1alpha1.SerializationFormatJSON,
				},
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		// Seed a non-zero generation: real inventories always carry one (k8s
		// assigns it), and generation,omitempty would drop a zero, defeating the
		// per-part generation-identity assertion.
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-demo"),
			MaxExportBytes:   &limit,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	backend := &partFailingBackend{failOnCall: 2}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return backend, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Read the resource back to learn the fake-client-assigned generation.
	var got kollectdevv1alpha1.KollectInventory
	invKey := types.NamespacedName{Name: "team-inventory", Namespace: "default"}
	if err := cl.Get(context.Background(), invKey, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	if got.Generation == 0 {
		t.Fatal("expected non-zero generation so the completeness marker carries generation identity")
	}

	// Confirm this input really is a three-part export.
	parts, err := export.PartitionEnvelopes(
		store.SnapshotNamespace("default"),
		export.Metadata{Generation: got.Generation},
		limit,
	)
	if err != nil {
		t.Fatalf("PartitionEnvelopes: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("computed parts = %d, want 3 (adjust item/limit sizing)", len(parts))
	}

	// Part 2 failed, so only part 1 was persisted before the goroutine broke.
	if len(backend.exported) != 1 {
		t.Fatalf("persisted parts = %d, want 1 (part 2 fails, part 3 never attempted)", len(backend.exported))
	}

	meta := export.EnvelopeMetaFromPayload(backend.exported[0])
	if meta.PartIndex != 1 || meta.PartTotal != 3 {
		t.Fatalf("persisted part markers = index %d total %d, want 1/3 (torn set must be detectable)",
			meta.PartIndex, meta.PartTotal)
	}
	if meta.Generation != got.Generation {
		t.Fatalf("persisted part generation = %d, want %d (generation identity per part)",
			meta.Generation, got.Generation)
	}

	// The resource must report the failure, not success.
	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue {
		t.Fatalf("Degraded condition = %+v, want True (incomplete export must not read as success)", degraded)
	}
	synced := apimeta.FindStatusCondition(got.Status.Conditions, conditionSinkSynced)
	if synced != nil && synced.Status == metav1.ConditionTrue {
		t.Fatalf("Synced condition = %+v, want not-True on a torn export", synced)
	}
}
