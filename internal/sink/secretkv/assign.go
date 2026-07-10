// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package secretkv

// AssignIfPresent copies the raw value stored under key in data into *dest,
// but only when key is present in data. The value is assigned verbatim (no
// trimming) — callers in the event-sink family (nats, kafka) resolve optional
// auth credentials (token/username/password) this way.
//
// A key that is present with an empty value still overwrites *dest; this is
// deliberate so that iterating several keys into the same destination lets a
// later present key override an earlier one (kafka folds both "password" and
// "token" into Password, token last). Absent keys leave *dest untouched, so
// it retains its zero or prior value. data may be nil.
func AssignIfPresent(data map[string][]byte, key string, dest *string) {
	if v, ok := data[key]; ok {
		*dest = string(v)
	}
}
