//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/nats"
)

func TestNATSTransportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := nats.Run(ctx, "nats:2.11")
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start nats: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pub, sub, err := NewTransport(Config{
		Type: TypeNATS,
		NATS: NATSConfig{URL: url},
		Group: "kollect-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = Close(pub)
		_ = Close(sub)
	}()

	rtCtx, rtCancel := context.WithTimeout(ctx, 45*time.Second)
	defer rtCancel()

	if err := RoundTrip(rtCtx, struct {
		Publisher
		Subscriber
	}{pub, sub}, "inventory/reports", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}
