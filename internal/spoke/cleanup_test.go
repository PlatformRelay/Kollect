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
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestPublishInventoryDeletion(t *testing.T) {
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
		_, _, _, err := hub.ReceiveReport("", payload, merger, nil, false)

		return err
	}); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 2},
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
		t.Fatalf("publish: %v", err)
	}

	if hubStore.TotalCount() != 1 {
		t.Fatalf("hub count = %d, want 1", hubStore.TotalCount())
	}

	var lastPayload []byte
	if err := bus.Subscribe(ctx, "inventory/reports", func(_ context.Context, payload []byte) error {
		lastPayload = append([]byte(nil), payload...)

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := PublishInventoryDeletion(ctx, store, inv); err != nil {
		t.Fatalf("PublishInventoryDeletion: %v", err)
	}

	var report hub.SpokeReport
	if err := json.Unmarshal(lastPayload, &report); err != nil {
		t.Fatal(err)
	}

	if len(report.RemovedUIDs) != 1 || report.RemovedUIDs[0] != "uid-1" {
		t.Fatalf("RemovedUIDs = %v, want [uid-1]", report.RemovedUIDs)
	}

	if len(report.Items) != 0 {
		t.Fatalf("Items = %d, want 0", len(report.Items))
	}
}

func TestPublishInventoryDeletionNoOpWithoutEnv(t *testing.T) {
	resetPublishState()
	t.Cleanup(resetPublishState)

	store := collect.NewStore()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv"},
	}

	if err := PublishInventoryDeletion(context.Background(), store, inv); err != nil {
		t.Fatalf("PublishInventoryDeletion: %v", err)
	}
}
