// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

func TestInventoryFromObjectPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path    string
		wantNS  string
		wantInv string
		wantErr bool
	}{
		{path: "inventory/team-a/rollup.json", wantNS: "team-a", wantInv: "rollup"},
		{path: "/inventory/ns/name.json", wantNS: "ns", wantInv: "name"},
		{path: "exports/latest.json", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			ns, name, err := inventoryFromObjectPath(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("inventoryFromObjectPath: %v", err)
			}
			if ns != tc.wantNS || name != tc.wantInv {
				t.Fatalf("got %q/%q, want %q/%q", ns, name, tc.wantNS, tc.wantInv)
			}
		})
	}
}

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
