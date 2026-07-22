// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import (
	"testing"
)

func TestResolveMode(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		envMode string
	}{
		{name: "empty"},
		{name: "flag cluster", flag: "cluster"},
		{name: "flag single", flag: "single"},
		{name: "flag mixed case", flag: "ClUsTeR"},
		{name: "flag whitespace", flag: "  single\t"},
		{name: "flag unknown", flag: "mystery"},
		{name: "environment cluster", envMode: "cluster"},
		{name: "environment single", envMode: "single"},
		{name: "environment mixed case", envMode: "SiNgLe"},
		{name: "environment whitespace", envMode: "  cluster\n"},
		{name: "environment unknown", envMode: "mystery"},
		{name: "flag takes precedence", flag: "unknown", envMode: "single"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envMode, tc.envMode)

			if got := ResolveMode(tc.flag); got != ModeCluster {
				t.Fatalf("ResolveMode(%q) with %s=%q = %q, want %q", tc.flag, envMode, tc.envMode, got, ModeCluster)
			}
		})
	}
}
