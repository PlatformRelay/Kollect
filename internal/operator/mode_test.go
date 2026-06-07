// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import (
	"testing"
)

func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		flag string
		want string
	}{
		{"", ModeCluster},
		{"single", ModeCluster},
		{"cluster", ModeCluster},
	}

	for _, tc := range tests {
		if got := ResolveMode(tc.flag); got != tc.want {
			t.Fatalf("ResolveMode(%q) = %q, want %q", tc.flag, got, tc.want)
		}
	}
}

func TestResolveModeFromEnv(t *testing.T) {
	t.Setenv(envMode, "cluster")
	if got := ResolveMode(""); got != ModeCluster {
		t.Fatalf("env mode = %q", got)
	}
}

func TestResolveModeUnknownFallsBackToCluster(t *testing.T) {
	t.Parallel()

	if got := ResolveMode("mystery"); got != ModeCluster {
		t.Fatalf("unknown mode = %q", got)
	}
}
