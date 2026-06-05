// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestTryPublishReportDeltaInProcess(t *testing.T) {
	resetPublisherCache()
	resetPublishState()
	t.Cleanup(func() {
		resetPublisherCache()
		resetPublishState()
	})

	t.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "inprocess")

	store := collect.NewStore()
	hubStore := collect.NewStore()
	merger := hub.NewMerger(hubStore)

	bus := transport.NewInProcessBus()
	testPublisher = bus

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bus.Subscribe(ctx, "inventory/reports", func(_ context.Context, payload []byte) error {
		_, _, err := hub.ReceiveReport("", payload, merger, nil, false)

		return err
	}); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 1},
	}

	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "nginx",
		Namespace:       "apps",
		Name:            "demo",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	if hubStore.TotalCount() != 1 {
		t.Fatalf("hub count = %d, want 1", hubStore.TotalCount())
	}

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("coalesced publish: %v", err)
	}

	inv.Generation = 2
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "nginx",
		Namespace:       "apps",
		Name:            "demo-2",
		UID:             "uid-2",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("delta publish: %v", err)
	}

	if hubStore.TotalCount() != 2 {
		t.Fatalf("hub count after delta = %d, want 2", hubStore.TotalCount())
	}
}

func TestTryPublishReportRemovedUIDsDelta(t *testing.T) {
	resetPublisherCache()
	resetPublishState()
	t.Cleanup(func() {
		resetPublisherCache()
		resetPublishState()
	})

	t.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "inprocess")

	store := collect.NewStore()
	var lastPayload []byte

	bus := transport.NewInProcessBus()
	testPublisher = bus

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bus.Subscribe(ctx, "inventory/reports", func(_ context.Context, payload []byte) error {
		lastPayload = append([]byte(nil), payload...)

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 1},
	}
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "t",
		Namespace:       "apps",
		Name:            "a",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatal(err)
	}

	inv.Generation = 2
	store.Remove("team-a", "t", "uid-1")

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatal(err)
	}

	var report hub.SpokeReport
	if err := json.Unmarshal(lastPayload, &report); err != nil {
		t.Fatal(err)
	}

	if report.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", report.SchemaVersion, export.SchemaVersion)
	}

	if len(report.Items) != 0 {
		t.Fatalf("items = %d, want 0 delta items", len(report.Items))
	}

	if len(report.RemovedUIDs) != 1 || report.RemovedUIDs[0] != "uid-1" {
		t.Fatalf("removed = %v", report.RemovedUIDs)
	}
}

func TestDeltaItemsOnlyChanged(t *testing.T) {
	prev := map[string]collect.Item{
		"u1": {UID: "u1", Name: "a", Version: "v1", Kind: "Pod"},
		"u2": {UID: "u2", Name: "b", Version: "v1", Kind: "Pod"},
	}
	current := []collect.Item{
		{UID: "u1", Name: "a", Version: "v1", Kind: "Pod"},
		{UID: "u2", Name: "b-changed", Version: "v1", Kind: "Pod"},
		{UID: "u3", Name: "c", Version: "v1", Kind: "Pod"},
	}

	changed, removed := deltaItems(prev, current, true)
	if len(removed) != 0 {
		t.Fatalf("removed = %v", removed)
	}

	if len(changed) != 2 {
		t.Fatalf("changed = %d, want 2 (u2 updated, u3 new)", len(changed))
	}
}
