// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestContentHash_matchesSHA256Hex(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"items":[{"name":"web"}]}`)
	want := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(want[:])

	if got := ContentHash(payload); got != wantHex {
		t.Fatalf("ContentHash = %q, want %q", got, wantHex)
	}
}

func TestContentHash_emptyPayload(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256(nil)
	want := hex.EncodeToString(sum[:])
	if got := ContentHash(nil); got != want {
		t.Fatalf("ContentHash(nil) = %q, want %q", got, want)
	}
}
