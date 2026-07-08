// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

// inventoryFromObjectPath behavior is now centrally tested in
// internal/pathvalidate (TestInventoryFromObjectPath).

func TestNewBackendAndType(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://gitlab.example.com/platform/inventory.git",
	}
	b, err := NewBackend(spec, nil, git.Auth{Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Config().Endpoint != spec.Endpoint {
		t.Fatalf("Config() = %#v", b.Config())
	}
}
