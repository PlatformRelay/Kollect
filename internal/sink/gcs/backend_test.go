// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

func TestNewBackend_wrongType(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "s3"}, nil)
	if err == nil {
		t.Fatal("expected error for non-gcs type")
	}
}

func TestNewBackend_missingEndpoint(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName}, nil)
	if err == nil {
		t.Fatal("expected error without endpoint")
	}
}

func TestNewBackend_valid(t *testing.T) {
	t.Parallel()

	// NewBackend construction is pure: it clones the spec to an s3 sink and
	// builds the inner client lazily (no network). A bucket-bearing endpoint is
	// enough for ConfigFromSpec to succeed.
	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://storage.googleapis.com/my-bucket/inventory",
	}, map[string][]byte{
		"accessKeyID":     []byte("a"),
		"secretAccessKey": []byte("b"),
	})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	if b == nil {
		t.Fatal("NewBackend returned nil backend")
	}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q, want %q", b.Type(), TypeName)
	}
}

func TestBackend_TypeAndCapabilities(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Capabilities() != cap.ObjectStoreSnapshot() {
		t.Fatalf("Capabilities() = %#v", b.Capabilities())
	}
}
