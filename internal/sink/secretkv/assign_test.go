// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package secretkv

import "testing"

// TestAssignIfPresent_SetsRawValueWhenKeyPresent backs the dup-audit
// extraction of the nats + kafka event-sink secret->auth-config scan: each
// copied the raw (untrimmed) value of a secret key into a config field when
// the key was present. Unlike FirstValue, the value is NOT trimmed — the
// event sinks assigned string(v) verbatim.
func TestAssignIfPresent_SetsRawValueWhenKeyPresent(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{"token": []byte("  secret-token  ")}

	dest := "prior"
	AssignIfPresent(data, "token", &dest)

	if dest != "  secret-token  " {
		t.Fatalf("dest = %q, want the raw untrimmed secret value", dest)
	}
}

// TestAssignIfPresent_PresentButEmptyOverwrites locks the kafka edge case:
// its loop over {"password", "token"} lets a *present* token key overwrite an
// earlier password even when token's value is empty. A helper that could not
// distinguish "key absent" from "key present but empty" would silently change
// this behaviour, so the extraction must preserve it.
func TestAssignIfPresent_PresentButEmptyOverwrites(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{"token": []byte("")}

	dest := "from-password"
	AssignIfPresent(data, "token", &dest)

	if dest != "" {
		t.Fatalf("dest = %q, want empty: a present-but-empty key overwrites", dest)
	}
}

// TestAssignIfPresent_AbsentKeyLeavesDestUntouched covers the common path:
// an optional credential key that is not in the secret leaves the config
// field at its zero/prior value.
func TestAssignIfPresent_AbsentKeyLeavesDestUntouched(t *testing.T) {
	t.Parallel()

	dest := "unchanged"

	AssignIfPresent(map[string][]byte{"other": []byte("v")}, "token", &dest)
	if dest != "unchanged" {
		t.Fatalf("dest = %q, want it untouched when key absent", dest)
	}

	AssignIfPresent(nil, "token", &dest)
	if dest != "unchanged" {
		t.Fatalf("dest = %q, want it untouched for nil data", dest)
	}
}

// TestAssignIfPresent_LastPresentKeyWins reproduces kafka's
// for-range{"password","token"} sequence: assigning the same destination from
// several keys in order leaves the last present key's value.
func TestAssignIfPresent_LastPresentKeyWins(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{
		"password": []byte("pw"),
		"token":    []byte("tok"),
	}

	dest := ""
	for _, key := range []string{"password", "token"} {
		AssignIfPresent(data, key, &dest)
	}

	if dest != "tok" {
		t.Fatalf("dest = %q, want the last present key (token) to win", dest)
	}
}
