// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import "testing"

func TestResolveProjectRef(t *testing.T) {
	ref, err := ResolveProjectRef("https://gitlab.example.com/platform/kollect-inventory.git")
	if err != nil {
		t.Fatalf("ResolveProjectRef: %v", err)
	}
	if ref.Path != "platform/kollect-inventory" {
		t.Fatalf("Path = %q, want platform/kollect-inventory", ref.Path)
	}
}
