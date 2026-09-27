// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package local

import (
	"os"
	"path/filepath"
	"testing"
)

// A candidate whose directory never existed is a clean no-op.
func TestBackend_DeleteExport_MissingDirIsNoop(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{OutputDir: t.TempDir()}}
	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-gone/apps.json"})
	if err != nil || deleted != nil {
		t.Fatalf("missing dir = %v/%v, want nil/nil", deleted, err)
	}
}

// A regular file where the matcher expects a directory is a real read error,
// not a missing directory, and must surface.
func TestBackend_DeleteExport_DirReadErrorPropagates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "inventory"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory", "team-b"), []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b := &Backend{cfg: Config{OutputDir: dir}}
	if _, err := b.DeleteExport(t.Context(), []string{"inventory/team-b/apps.json"}); err == nil {
		t.Fatal("a file where a directory is expected must error")
	}
}

// Subdirectories and non-matching files in the target directory are skipped;
// only the matching candidate is removed.
func TestBackend_DeleteExport_SkipsDirsAndNonMatching(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := filepath.Join(dir, "inventory", "team-a")
	if err := os.MkdirAll(filepath.Join(base, "subdir"), 0o750); err != nil {
		t.Fatalf("MkdirAll(subdir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "apps.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(apps): %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "apps-v2.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(apps-v2): %v", err)
	}

	b := &Backend{cfg: Config{OutputDir: dir}}
	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "inventory/team-a/apps.json" {
		t.Fatalf("deleted = %v, want the exact candidate only", deleted)
	}
	if _, statErr := os.Stat(filepath.Join(base, "subdir")); statErr != nil {
		t.Fatalf("subdirectory must survive: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(base, "apps-v2.json")); statErr != nil {
		t.Fatalf("non-matching sibling must survive: %v", statErr)
	}
}
