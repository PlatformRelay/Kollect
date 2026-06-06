// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestBackendConfigAndType(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
	}
	b, err := NewBackend(spec, nil, Auth{Token: "tok"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Config().Endpoint != spec.Endpoint {
		t.Fatalf("Config() = %#v", b.Config())
	}
	if b.Capabilities().Stream {
		t.Fatal("git snapshot should not be stream emitter")
	}
}
