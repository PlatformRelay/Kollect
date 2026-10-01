// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package local

import (
	"os"
	"path/filepath"
	"testing"
)

// K-28 test lock (tombstone): local sink cleanup removes the exported file and
// its part siblings, keeps sibling inventories, and is idempotent.
func TestBackend_DeleteExport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := &Backend{cfg: Config{OutputDir: dir}}

	files := map[string]bool{
		"inventory/team-a/apps.json":                   true,
		"inventory/team-a/apps.part-0001-of-0002.json": true,
		"inventory/team-a/apps.part-0002-of-0002.json": true,
		"inventory/team-a/apps-v2.json":                false,
		"inventory/team-b/apps.json":                   false,
	}
	for name := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted = %v, want the file and its two part siblings", deleted)
	}

	for name, wantGone := range files {
		_, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		gone := os.IsNotExist(statErr)
		if gone != wantGone {
			t.Errorf("%q deleted=%v, want deleted=%v", name, gone, wantGone)
		}
	}

	if deleted, err = b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"}); err != nil || len(deleted) != 0 {
		t.Fatalf("second DeleteExport = %v/%v, want empty/nil (idempotent)", deleted, err)
	}
}

// Cleanup must never reach outside the output directory.
func TestBackend_DeleteExport_RejectsEscape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := &Backend{cfg: Config{OutputDir: dir}}

	outside := filepath.Join(t.TempDir(), "victim.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := b.DeleteExport(t.Context(), []string{"../victim.json"}); err == nil {
		t.Fatal("DeleteExport accepted a traversal path")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside the output dir must survive: %v", err)
	}
}
