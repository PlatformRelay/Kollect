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

// InventoryFromObjectPath used to be duplicated byte-for-byte across the
// postgres, bigquery, and mongodb sink backends; this case set merges all
// behaviors previously exercised by their three independent test tables.
func TestInventoryFromObjectPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		wantNS   string
		wantName string
	}{
		{name: "namespace and name", path: "inventory/team-a/rollup.json", wantNS: "team-a", wantName: "rollup"},
		{name: "ns/name no special chars", path: "inventory/ns/name.json", wantNS: "ns", wantName: "name"},
		{name: "single segment treated as namespace", path: "inventory/solo.json", wantNS: "solo.json", wantName: ""},
		{name: "no inventory prefix", path: "exports/latest.json", wantNS: "exports", wantName: "latest"},
		{name: "empty path", path: "", wantNS: "", wantName: ""},
		{name: "trimmed surrounding spaces", path: "  inventory/team-b/workloads.json  ", wantNS: "team-b", wantName: "workloads"},
		{name: "namespace only, no file segment", path: "inventory/team-c", wantNS: "team-c", wantName: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotNS, gotName := InventoryFromObjectPath(tc.path)
			if gotNS != tc.wantNS || gotName != tc.wantName {
				t.Fatalf("InventoryFromObjectPath(%q) = (%q, %q), want (%q, %q)",
					tc.path, gotNS, gotName, tc.wantNS, tc.wantName)
			}
		})
	}
}
