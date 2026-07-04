// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package secretkv

import "testing"

// TestFirstValue_ReturnsFirstNonEmptyMatchInKeyOrder backs the dup-audit
// extraction of postgres's dsnFromSecret / mongodb's uriFromSecret: both
// scanned a secret for the first non-empty value among several candidate
// key names, in priority order.
func TestFirstValue_ReturnsFirstNonEmptyMatchInKeyOrder(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{
		"url": []byte("postgres://from-url"),
		"dsn": []byte("postgres://from-dsn"),
	}

	got, ok := FirstValue(data, "dsn", "url", "connectionString")
	if !ok {
		t.Fatal("expected a value")
	}
	if got != "postgres://from-dsn" {
		t.Fatalf("FirstValue = %q, want value from the first matching key (dsn)", got)
	}
}

func TestFirstValue_SkipsEmptyAndWhitespaceOnlyValues(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{
		"dsn": []byte("   "),
		"url": []byte("postgres://from-url"),
	}

	got, ok := FirstValue(data, "dsn", "url")
	if !ok {
		t.Fatal("expected a value, dsn key present but blank should be skipped")
	}
	if got != "postgres://from-url" {
		t.Fatalf("FirstValue = %q, want fallback to url", got)
	}
}

func TestFirstValue_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	got, ok := FirstValue(map[string][]byte{"dsn": []byte("  postgres://x  ")}, "dsn")
	if !ok || got != "postgres://x" {
		t.Fatalf("FirstValue = (%q, %v), want (\"postgres://x\", true)", got, ok)
	}
}

func TestFirstValue_NoMatchReturnsFalse(t *testing.T) {
	t.Parallel()

	_, ok := FirstValue(map[string][]byte{"other": []byte("v")}, "dsn", "url")
	if ok {
		t.Fatal("expected no match for unrelated keys")
	}

	_, ok = FirstValue(nil, "dsn")
	if ok {
		t.Fatal("expected no match for nil data")
	}
}
