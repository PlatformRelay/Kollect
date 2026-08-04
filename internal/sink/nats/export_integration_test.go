//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/digest"
	"github.com/platformrelay/kollect/internal/export"
)

func TestExportNATS(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	url := startNATSTestContainer(t)

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

	if envelope.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", envelope.SchemaVersion, export.SchemaVersion)
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

// TestExport_ReconnectsAfterConnectionClosed verifies that a Backend whose
// cached connection has been closed (simulating an exhausted reconnect after
// the server dropped) transparently re-establishes the connection on the next
// Export instead of failing permanently.
func TestExport_ReconnectsAfterConnectionClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	url := startNATSTestContainer(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "nats",
		Cluster: "test-cluster",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     url,
			Subject: "inventory.reconnect",
			Stream:  "kollect_reconnect_events",
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
		t.Fatalf("initial Export: %v", err)
	}

	// Simulate a permanently dropped connection: close the cached conn directly.
	backend.mu.Lock()
	if backend.nc == nil {
		backend.mu.Unlock()
		t.Fatal("expected a cached connection after first Export")
	}
	backend.nc.Close()
	closedSeen := backend.nc.IsClosed()
	backend.mu.Unlock()

	if !closedSeen {
		t.Fatal("expected cached connection to report IsClosed after Close")
	}

	// The next Export must re-establish the connection and succeed.
	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export after connection closed: %v", err)
	}

	backend.mu.Lock()
	reconnected := backend.nc != nil && !backend.nc.IsClosed()
	backend.mu.Unlock()
	if !reconnected {
		t.Fatal("expected backend to hold a live connection after reconnect")
	}
}

// TestExport_ensureStreamProvisionsFreshStream verifies Export creates a missing
// JetStream stream (ensureStream) and publishes with the expected subject + Msg-Id
// (COV-90-S12 / Track B).
func TestExport_ensureStreamProvisionsFreshStream(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	url := startNATSTestContainer(t)

	const (
		subject    = "inventory.ensure"
		streamName = "kollect_ensure_stream"
	)

	nc, err := connect(Config{URL: url}, TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := js.Stream(readCtx, streamName); err == nil {
		t.Fatalf("stream %q must not exist before Export", streamName)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "nats",
		Cluster: "ensure-cluster",
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
	t.Cleanup(func() { _ = backend.Close() })

	payload := []byte(`[{"namespace":"apps","uid":"uid-ensure"}]`)
	if err := backend.Export(ctx, payload, "inventory/apps/ensure.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	stream, err := js.Stream(readCtx, streamName)
	if err != nil {
		t.Fatalf("stream after Export: %v", err)
	}
	info, err := stream.Info(readCtx)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != subject {
		t.Fatalf("stream subjects = %v, want [%s]", info.Config.Subjects, subject)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("stream messages = %d, want 1", info.State.Msgs)
	}

	cons, err := js.CreateOrUpdateConsumer(readCtx, streamName, jetstream.ConsumerConfig{
		Durable:       "kollect-test-ensure",
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	var gotMsg jetstream.Msg
	consumeCtx, err := cons.Consume(func(msg jetstream.Msg) { gotMsg = msg })
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
	if gotMsg.Subject() != subject {
		t.Fatalf("message subject = %q, want %q", gotMsg.Subject(), subject)
	}

	wantMsgID := digest.ContentHash(append([]byte("ensure-cluster/apps/"), payload...))
	if hdr := gotMsg.Headers(); hdr == nil || hdr.Get("Nats-Msg-Id") != wantMsgID {
		t.Fatalf("Nats-Msg-Id = %q, want %q", hdr.Get("Nats-Msg-Id"), wantMsgID)
	}
}

// TestExport_transientDisconnectIdempotentRetry simulates a dropped connection
// (transient publish path failure) and asserts the subsequent Export reconnects
// and re-publishes the same Msg-Id without duplicating stream messages
// (COV-90-S12 / Track B).
func TestExport_transientDisconnectIdempotentRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	url := startNATSTestContainer(t)

	const (
		subject    = "inventory.retry"
		streamName = "kollect_retry_events"
	)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "nats",
		Cluster: "retry-cluster",
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
	t.Cleanup(func() { _ = backend.Close() })

	payload := []byte(`[{"namespace":"apps","uid":"uid-retry"}]`)
	if err := backend.Export(ctx, payload, "inventory/apps/retry.json"); err != nil {
		t.Fatalf("initial Export: %v", err)
	}

	backend.mu.Lock()
	if backend.nc == nil {
		backend.mu.Unlock()
		t.Fatal("expected cached connection after initial Export")
	}
	backend.nc.Close()
	backend.mu.Unlock()

	if err := backend.Export(ctx, payload, "inventory/apps/retry.json"); err != nil {
		t.Fatalf("Export after transient disconnect: %v", err)
	}

	nc, err := connect(Config{URL: url}, TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stream, err := js.Stream(readCtx, streamName)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	info, err := stream.Info(readCtx)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("stream messages after idempotent retry = %d, want 1 (Msg-Id dedupe)", info.State.Msgs)
	}
}

// TestExport_ensureStreamSubjectConflict asserts Export fails when JetStream
// refuses CreateOrUpdateStream because the subject is already owned by another
// stream (COV-90-S12 / Track B stream error).
func TestExport_ensureStreamSubjectConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	url := startNATSTestContainer(t)

	const subject = "inventory.conflict"

	nc, err := connect(Config{URL: url}, TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := js.CreateOrUpdateStream(readCtx, jetstream.StreamConfig{
		Name:     "foreign_owner",
		Subjects: []string{subject},
	}); err != nil {
		t.Fatalf("pre-create foreign stream: %v", err)
	}

	backend, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:    "nats",
		Cluster: "conflict-cluster",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     url,
			Subject: subject,
			Stream:  "kollect_conflict_stream",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	err = backend.Export(ctx, []byte(`[{"uid":"x"}]`), "inventory/apps/conflict.json")
	if err == nil {
		t.Fatal("expected Export to fail on stream subject conflict")
	}
	if !strings.Contains(err.Error(), "nats create stream:") {
		t.Fatalf("error = %q, want 'nats create stream:' prefix", err)
	}
}
