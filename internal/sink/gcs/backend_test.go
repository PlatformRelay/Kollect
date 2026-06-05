// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestNewBackendRejectsWrongType(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "s3"}, nil)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestNewBackendAndType(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     typeName,
		Endpoint: "https://storage.googleapis.com/my-bucket",
	}
	b, err := NewBackend(spec, map[string][]byte{
		"accessKeyID":     []byte("key"),
		"secretAccessKey": []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Type() != typeName {
		t.Fatalf("Type() = %q", b.Type())
	}

	if !b.Capabilities().ObjectStore {
		t.Fatal("expected object store capabilities")
	}
}
