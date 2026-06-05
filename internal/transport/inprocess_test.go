// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestInProcessBusPublishSubscribe(t *testing.T) {
	t.Parallel()

	bus := NewInProcessBus()
	var count atomic.Int32

	_ = bus.Subscribe(context.Background(), "inventory/default", func(_ context.Context, payload []byte) error {
		count.Add(1)
		if string(payload) != "ping" {
			t.Errorf("payload = %q", payload)
		}

		return nil
	})

	if err := bus.Publish(context.Background(), "inventory/default", []byte("ping")); err != nil {
		t.Fatal(err)
	}

	if count.Load() != 1 {
		t.Fatalf("handler calls = %d", count.Load())
	}
}
