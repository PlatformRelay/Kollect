// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import "testing"

// inventoryFromObjectPath behavior is now centrally tested in
// internal/pathvalidate (TestInventoryFromObjectPath).

func TestQualifiedTable_escapesBackticks(t *testing.T) {
	t.Parallel()

	got := qualifiedTable("proj`x", "data`set", "tab`le")
	if got != "`projx.dataset.table`" {
		t.Fatalf("qualifiedTable = %q", got)
	}
}

func TestQualifiedTable(t *testing.T) {
	t.Parallel()

	got := qualifiedTable("proj", "dataset", "items")
	if got != "`proj.dataset.items`" {
		t.Fatalf("qualifiedTable = %q", got)
	}
}
