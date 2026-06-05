// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestInProcessPublishWireClusterRoundTrip(t *testing.T) {
	t.Parallel()

	bus := transport.NewInProcessBus()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan string, 1)
	if err := bus.SubscribeWire(ctx, "inventory/reports", func(_ context.Context, wireCluster string, _ []byte) error {
		done <- wireCluster

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{Cluster: "spoke-a"}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	pubCtx := transport.WithWireClusterID(ctx, "spoke-a")
	if err := bus.Publish(pubCtx, "inventory/reports", payload); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("in-process wire cluster = %q, want empty", got)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}
