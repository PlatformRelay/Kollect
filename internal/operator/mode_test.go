// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import "testing"

func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		flag string
		hub  bool
		want string
	}{
		{"", false, ModeCluster},
		{"single", false, ModeCluster},
		{"hub", false, ModeHub},
		{"", true, ModeHub},
		{"spoke", false, ModeSpoke},
	}

	for _, tc := range tests {
		if got := ResolveMode(tc.flag, tc.hub); got != tc.want {
			t.Fatalf("ResolveMode(%q, %v) = %q, want %q", tc.flag, tc.hub, got, tc.want)
		}
	}
}
