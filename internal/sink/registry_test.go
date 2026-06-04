// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestRegistry_NewBackend(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	gitBackend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "git",
		Endpoint: "https://example.com/inventory.git",
	})
	if err != nil {
		t.Fatalf("NewBackend(git) error = %v", err)
	}

	if gitBackend.Type() != "git" {
		t.Fatalf("Type() = %q, want git", gitBackend.Type())
	}

	if err := gitBackend.Export(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if _, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}); err == nil {
		t.Fatal("NewBackend(unknown) expected error")
	}
}
