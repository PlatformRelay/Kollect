// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestInProcessHubMergeRoundTrip(t *testing.T) {
	t.Parallel()

	bus := transport.NewInProcessBus()
	store := collect.NewStore()
	merger := hub.NewMerger(store)
	consumer := hub.NewConsumer(bus, merger, "inventory/reports", "test-hub", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-1",
		InventoryRef: hub.InventoryRef{
			Namespace: "default",
			Name:      "team-inventory",
		},
		Items: []collect.Item{
			{Namespace: "apps", Name: "web", UID: "uid-web", Version: "v1", Kind: "Deployment"},
		},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, "inventory/reports", payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for store.TotalCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if store.TotalCount() != 1 {
		t.Fatalf("store count = %d, want 1", store.TotalCount())
	}
}
