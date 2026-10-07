// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"errors"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

func TestNamespaceFromObjectPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{path: "inventory/team-a/rollup.json", want: "team-a"},
		{path: "inventory/ns/name.json", want: "ns"},
		{path: "exports/latest.json", want: "exports"},
		{path: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			if got := namespaceFromObjectPath(tc.path); got != tc.want {
				t.Fatalf("namespaceFromObjectPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestStreamSubjects_empty(t *testing.T) {
	t.Parallel()

	if got := streamSubjects("  "); got != nil {
		t.Fatalf("streamSubjects empty = %v, want nil", got)
	}
}

func TestBackend_TypeCapabilitiesClose(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Cluster: "local", Subject: "inventory.events"}}
	if b.Type() != typeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Capabilities() != cap.StreamEmitter() {
		t.Fatalf("Capabilities() = %#v", b.Capabilities())
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBackend_Export_rejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Cluster: "local", Subject: "inventory.events"}}
	err := b.Export(context.Background(), nil, "inventory/default/inv.json")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestBackend_Export_afterCloseFailsWithoutRedialling(t *testing.T) {
	t.Cleanup(resetJetStreamFromConn)
	fjs := &fakeJetStream{}
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) { return fjs, nil }

	dials := 0
	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events", Stream: "events"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	b.connectFn = func(Config, TLSConfig) (*natsgo.Conn, error) {
		dials++
		return nil, nil
	}

	payload := []byte(`[{"uid":"u1"}]`)
	err = b.Export(context.Background(), payload, "inventory/apps/demo.json")
	if err != nil {
		t.Fatalf("first Export: %v", err)
	}
	if dials != 1 {
		t.Fatalf("dials after first Export = %d, want 1", dials)
	}
	err = b.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	err = b.Export(context.Background(), payload, "inventory/apps/demo.json")
	if !errors.Is(err, errBackendClosed) {
		t.Fatalf("Export after Close = %v, want errBackendClosed", err)
	}
	if dials != 1 {
		t.Fatalf("dials = %d, want 1 (no new connection after Close)", dials)
	}
}

func TestBackend_Close_beforeFirstConnectLatchesClosed(t *testing.T) {
	t.Cleanup(resetJetStreamFromConn)
	jetStreamFromConn = func(*natsgo.Conn) (jetstream.JetStream, error) { return &fakeJetStream{}, nil }

	dials := 0
	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events", Stream: "events"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	b.connectFn = func(Config, TLSConfig) (*natsgo.Conn, error) {
		dials++
		return nil, nil
	}

	err = b.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	err = b.Export(context.Background(), []byte(`{"x":1}`), "inventory/apps/demo.json")
	if !errors.Is(err, errBackendClosed) {
		t.Fatalf("Export after Close-before-connect = %v, want errBackendClosed", err)
	}
	if dials != 0 {
		t.Fatalf("dials = %d, want 0 (Close must latch before any dial)", dials)
	}
}

func TestNewBackend_invalidSpec(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "nats"}, nil, nil)
	if err == nil {
		t.Fatal("expected error without nats spec")
	}
}

func TestNewBackend_validSpec(t *testing.T) {
	t.Parallel()

	backend, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     "nats://broker:4222",
			Subject: "inventory.events",
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	if backend.Type() != typeName {
		t.Fatalf("Type() = %q", backend.Type())
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBackend_Export_usesJetStreamProvider(t *testing.T) {
	t.Parallel()

	fjs := &fakeJetStream{}
	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{URL: "nats://broker:4222", Subject: "inventory.events", Stream: "events"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	b.jsProvider = func(context.Context) (jetstream.JetStream, error) { return fjs, nil }

	if err := b.Export(context.Background(), []byte(`{"x":1}`), "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if fjs.lastSubject != "inventory.events" {
		t.Fatalf("subject = %q", fjs.lastSubject)
	}
}
