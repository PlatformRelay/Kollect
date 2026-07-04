// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package secretkv resolves a connection value from a Kubernetes Secret's
// data when the field may live under any of several legacy/alternate key
// names (e.g. "dsn", "url", "connectionString"). Shared by the database
// family sink backends to avoid each one repeating the same scan loop.
package secretkv

import "strings"

// FirstValue returns the first non-empty, trimmed value found in data for
// any of the given candidate keys, in order. ok is false if no candidate key
// had a non-empty value.
func FirstValue(data map[string][]byte, keys ...string) (value string, ok bool) {
	for _, key := range keys {
		v, present := data[key]
		if !present {
			continue
		}

		trimmed := strings.TrimSpace(string(v))
		if trimmed != "" {
			return trimmed, true
		}
	}

	return "", false
}
