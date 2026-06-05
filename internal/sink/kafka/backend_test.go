// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
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

func TestDialTransport_withoutSASL(t *testing.T) {
	t.Parallel()

	transport, err := dialTransport(Config{})
	if err != nil {
		t.Fatalf("dialTransport: %v", err)
	}
	if transport == nil || transport.SASL != nil {
		t.Fatal("expected transport without SASL")
	}
}

func TestDialTransport_withSASL(t *testing.T) {
	t.Parallel()

	transport, err := dialTransport(Config{Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("dialTransport: %v", err)
	}
	if transport == nil || transport.SASL == nil {
		t.Fatal("expected SASL mechanism")
	}
}

func TestBackend_TypeCapabilitiesClose(t *testing.T) {
	t.Parallel()

	b := &Backend{
		cfg:    Config{Topic: "inventory", Cluster: "local"},
		writer: &kafka.Writer{},
	}
	if b.Type() != typeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Capabilities() != cap.StreamEmitter() {
		t.Fatalf("Capabilities() = %#v", b.Capabilities())
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := (&Backend{}).Close(); err != nil {
		t.Fatalf("Close nil writer: %v", err)
	}
}

func TestBackend_Export_rejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Cluster: "local"}}
	err := b.Export(context.Background(), nil, "inventory/default/inv.json")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestNewBackend_invalidSpec(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "kafka"}, nil)
	if err == nil {
		t.Fatal("expected error without kafka spec")
	}
}
