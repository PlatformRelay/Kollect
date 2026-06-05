// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"testing"

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
