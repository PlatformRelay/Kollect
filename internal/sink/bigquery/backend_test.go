// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import "testing"

func TestInventoryFromObjectPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path    string
		wantNS  string
		wantInv string
	}{
		{path: "inventory/team-a/rollup.json", wantNS: "team-a", wantInv: "rollup"},
		{path: "inventory/ns/name.json", wantNS: "ns", wantInv: "name"},
		{path: "inventory/solo.json", wantNS: "solo.json", wantInv: ""},
		{path: "exports/latest.json", wantNS: "exports", wantInv: "latest"},
		{path: "", wantNS: "", wantInv: ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			ns, name := inventoryFromObjectPath(tc.path)
			if ns != tc.wantNS || name != tc.wantInv {
				t.Fatalf("inventoryFromObjectPath(%q) = %q/%q, want %q/%q",
					tc.path, ns, name, tc.wantNS, tc.wantInv)
			}
		})
	}
}

func TestQualifiedTable(t *testing.T) {
	t.Parallel()

	got := qualifiedTable("proj", "dataset", "items")
	if got != "`proj.dataset.items`" {
		t.Fatalf("qualifiedTable = %q", got)
	}
}
