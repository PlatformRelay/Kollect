//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go/modules/nats"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/digest"

	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportNATS(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, err := nats.Run(ctx, "nats:2.11")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
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

	const (
		subject    = "inventory.events"
		streamName = "kollect_test_events"
	)
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "nats",
		Cluster: "test-cluster",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     url,
			Subject: subject,
			Stream:  streamName,
		},
	}

	backend, err := NewBackend(spec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})

	payload := []byte(`[{"namespace":"apps","uid":"uid-1"}]`)
	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	nc, err := connect(Config{URL: url}, TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		nc.Close()
	})

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cons, err := js.CreateOrUpdateConsumer(readCtx, streamName, jetstream.ConsumerConfig{
		Durable:       "kollect-test-export",
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	var gotMsg jetstream.Msg
	consumeCtx, err := cons.Consume(func(msg jetstream.Msg) {
		gotMsg = msg
	})
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	defer consumeCtx.Stop()

	deadline := time.Now().Add(15 * time.Second)
	for gotMsg == nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	if gotMsg == nil {
		t.Fatal("timed out waiting for JetStream message")
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(gotMsg.Data(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.Cluster != "test-cluster" {
		t.Fatalf("cluster = %q, want test-cluster", envelope.Cluster)
	}

	if envelope.Namespace != "apps" {
		t.Fatalf("namespace = %q, want apps", envelope.Namespace)
	}

	if string(envelope.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", envelope.Payload, payload)
	}

	wantMsgID := digest.ContentHash(append([]byte("test-cluster/apps/"), payload...))
	if hdr := gotMsg.Headers(); hdr == nil || hdr.Get("Nats-Msg-Id") != wantMsgID {
		t.Fatalf("Nats-Msg-Id = %q, want %q", hdr.Get("Nats-Msg-Id"), wantMsgID)
	}

	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("duplicate Export: %v", err)
	}

	info, err := js.Stream(readCtx, streamName)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}

	si, err := info.Info(readCtx)
	if err != nil {
		t.Fatalf("stream state: %v", err)
	}

	if si.State.Msgs != 1 {
		t.Fatalf("stream messages = %d, want 1 after duplicate export (Msg-Id dedupe)", si.State.Msgs)
	}
}
