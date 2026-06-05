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
	}, BuildContext{})
	if err != nil {
		t.Fatalf("NewBackend(git) error = %v", err)
	}

	if gitBackend.Type() != "git" {
		t.Fatalf("Type() = %q, want git", gitBackend.Type())
	}

	s3Backend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://inventory-bucket/prefix",
	}, BuildContext{
		SecretData: map[string][]byte{
			"accessKeyID":     []byte("key"),
			"secretAccessKey": []byte("secret"),
		},
	})
	if err != nil {
		t.Fatalf("NewBackend(s3) error = %v", err)
	}

	if s3Backend.Type() != "s3" {
		t.Fatalf("Type() = %q, want s3", s3Backend.Type())
	}

	if _, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}, BuildContext{}); err == nil {
		t.Fatal("NewBackend(unknown) expected error")
	}

	_ = context.Background()
	_ = gitBackend
	_ = s3Backend
}
