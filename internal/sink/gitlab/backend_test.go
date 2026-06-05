// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import "testing"

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
