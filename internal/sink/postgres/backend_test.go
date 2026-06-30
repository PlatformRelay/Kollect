// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import "testing"

// inventoryFromObjectPath behavior is now centrally tested in
// internal/pathvalidate (TestInventoryFromObjectPath); this package only
// needs to confirm it is wired into Export, which TestBackend_Export_*
// exercises indirectly.

func TestPgxQuoteIdent(t *testing.T) {
	t.Parallel()

	if got := pgxQuoteIdent("public"); got != `"public"` {
		t.Fatalf("quote = %q", got)
	}
	if got := pgxQuoteIdent(`weird"name`); got != `"weird""name"` {
		t.Fatalf("escaped quote = %q", got)
	}
}
