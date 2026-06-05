// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
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
