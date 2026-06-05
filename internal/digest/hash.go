// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package digest

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContentHash returns a SHA-256 hex digest of an export payload.
func ContentHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
