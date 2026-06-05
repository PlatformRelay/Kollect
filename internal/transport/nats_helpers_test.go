// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import "testing"

func TestSanitizeConsumerToken(t *testing.T) {
	t.Parallel()

	got := sanitizeConsumerToken("inventory/team-a/reports")
	want := "inventory_team_a_reports"
	if got != want {
		t.Fatalf("sanitized = %q, want %q", got, want)
	}

	if sanitizeConsumerToken("simple") != "simple" {
		t.Fatal("simple subject should pass through")
	}
}
