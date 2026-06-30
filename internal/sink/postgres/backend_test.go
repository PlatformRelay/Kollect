// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestBackend_Type(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q, want %q", b.Type(), TypeName)
	}
}

func TestBackend_Close_nilPoolIsNoop(t *testing.T) {
	t.Parallel()

	b := &Backend{pool: nil}
	b.Close() // must not panic
}

func TestBackend_Export_decodeError(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	err := b.Export(context.Background(), []byte(`{"schemaVersion":"kollect.dev/v99","items":[]}`), "")
	if err == nil {
		t.Fatal("expected decode error for unsupported schema version")
	}
	if !strings.Contains(err.Error(), "decode payload") {
		t.Fatalf("error = %q, want decode payload context", err)
	}
}

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
