// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pathvalidate

import "testing"

func TestValidateRelativeObjectPath_rejectsTraversal(t *testing.T) {
	t.Parallel()

	cases := []string{
		"../../../etc/passwd",
		"inventory/../../outside.json",
		"/etc/passwd",
		"inventory/latest.json\x00.evil",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateRelativeObjectPath(tc); err == nil {
				t.Fatalf("ValidateRelativeObjectPath(%q) = nil, want error", tc)
			}
		})
	}
}

func TestValidateRelativeObjectPath_acceptsSafePaths(t *testing.T) {
	t.Parallel()

	got, err := ValidateRelativeObjectPath("inventory/team-a/deployments.json")
	if err != nil {
		t.Fatalf("ValidateRelativeObjectPath() error = %v", err)
	}

	if got != "inventory/team-a/deployments.json" {
		t.Fatalf("got %q", got)
	}
}

func TestRejectTraversal(t *testing.T) {
	t.Parallel()

	if err := RejectTraversal("inventory/{namespace}/{name}.json"); err != nil {
		t.Fatalf("safe path: %v", err)
	}

	if err := RejectTraversal("../escape"); err == nil {
		t.Fatal("expected traversal error")
	}
}
