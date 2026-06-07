// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import "testing"

func TestExportFingerprintTracker_shouldSkip(t *testing.T) {
	t.Parallel()

	var tracker exportFingerprintTracker
	key := exportFingerprintKey("https://example.com/r.git", "main", "inventory/ns/inv.json")

	if tracker.shouldSkip(key, "abc") {
		t.Fatal("expected miss before record")
	}

	tracker.record(key, "abc")
	if !tracker.shouldSkip(key, "abc") {
		t.Fatal("expected skip for same checksum")
	}

	if tracker.shouldSkip(key, "def") {
		t.Fatal("expected export when checksum changes")
	}

	if tracker.shouldSkip(key, "") {
		t.Fatal("empty checksum must not skip")
	}
}
