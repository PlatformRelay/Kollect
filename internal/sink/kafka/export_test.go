// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/platformrelay/kollect/internal/export"
)

// fakeWriter is an injectable messageWriter used to unit-test Export without a
// real Kafka broker.
type fakeWriter struct {
	messages  []kafka.Message
	writeErr  error
	closeErr  error
	closed    bool
	writeCall int
}

func (f *fakeWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	f.writeCall++
	if f.writeErr != nil {
		return f.writeErr
	}
	f.messages = append(f.messages, msgs...)
	return nil
}

func (f *fakeWriter) Close() error {
	f.closed = true
	return f.closeErr
}

func TestExport_emptyPayloadDoesNotCallWriter(t *testing.T) {
	t.Parallel()

	fw := &fakeWriter{}
	b := &Backend{cfg: Config{Cluster: "local"}, writer: fw}

	err := b.Export(context.Background(), nil, "inventory/default/inv.json")
	if err == nil || !strings.Contains(err.Error(), "kafka export: empty payload") {
		t.Fatalf("expected empty payload error, got %v", err)
	}
	if fw.writeCall != 0 {
		t.Fatalf("writer was called %d times, want 0", fw.writeCall)
	}
}

func TestExport_publishesEnvelopeWithClusterNamespaceKey(t *testing.T) {
	t.Parallel()

	fw := &fakeWriter{}
	b := &Backend{cfg: Config{Cluster: "prod", Topic: "inventory"}, writer: fw}

	payload := []byte(`[{"uid":"u1"}]`)
	if err := b.Export(context.Background(), payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if len(fw.messages) != 1 {
		t.Fatalf("wrote %d messages, want 1", len(fw.messages))
	}
	msg := fw.messages[0]

	if got, want := string(msg.Key), "prod/apps"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}

	var env EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if env.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", env.SchemaVersion, export.SchemaVersion)
	}
	if env.Cluster != "prod" {
		t.Fatalf("cluster = %q, want prod", env.Cluster)
	}
	if env.Namespace != "apps" {
		t.Fatalf("namespace = %q, want apps", env.Namespace)
	}
	if string(env.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", env.Payload, payload)
	}
	if strings.TrimSpace(env.Timestamp) == "" {
		t.Fatal("timestamp is empty")
	}
}

func TestExport_keyFallsBackToObjectPathWhenClusterAndNamespaceEmpty(t *testing.T) {
	t.Parallel()

	fw := &fakeWriter{}
	// Empty cluster and an objectPath whose namespace segment is empty.
	b := &Backend{cfg: Config{Cluster: ""}, writer: fw}

	if err := b.Export(context.Background(), []byte(`{}`), "/only.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(fw.messages) != 1 {
		t.Fatalf("wrote %d messages, want 1", len(fw.messages))
	}
	if got := string(fw.messages[0].Key); got != "/only.json" {
		t.Fatalf("key = %q, want fallback objectPath %q", got, "/only.json")
	}
}

func TestExport_wrapsWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	fw := &fakeWriter{writeErr: sentinel}
	b := &Backend{cfg: Config{Cluster: "prod"}, writer: fw}

	err := b.Export(context.Background(), []byte(`{}`), "inventory/apps/x.json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error %v does not wrap sentinel", err)
	}
	if !strings.Contains(err.Error(), "kafka publish:") {
		t.Fatalf("error %q missing 'kafka publish:' prefix", err.Error())
	}
}

func TestClose_delegatesToWriter(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("close boom")
	fw := &fakeWriter{closeErr: sentinel}
	b := &Backend{writer: fw}

	if err := b.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("Close error = %v, want wrap of sentinel", err)
	}
	if !fw.closed {
		t.Fatal("Close did not delegate to writer")
	}
}
