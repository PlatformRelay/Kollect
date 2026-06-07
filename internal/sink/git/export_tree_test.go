// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initBareRepo(t *testing.T, dir string) string {
	t.Helper()

	bare := filepath.Join(dir, "repo.git")
	cmd := exec.Command("git", "init", "--bare", bare) //nolint:gosec // G204: test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	return bare
}

func cloneMain(t *testing.T, bare, dest string) {
	t.Helper()

	cmd := exec.Command("git", "clone", "--branch", "main", "--single-branch", bare, dest) //nolint:gosec // G204: test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %s: %v", out, err)
	}
}

func TestExportFilesWithBranch_writesTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := initBareRepo(t, dir)
	cfg := Config{Endpoint: "file://" + bare}

	files := []FileEntry{
		{Path: "prod-west/team-a/deployment/api.yaml", Data: []byte("kind: Deployment\nname: api\n")},
		{Path: "prod-west/team-a/deployment/web.yaml", Data: []byte("kind: Deployment\nname: web\n")},
	}

	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{Checksum: "c1"}); err != nil {
		t.Fatalf("ExportFilesWithBranch() error = %v", err)
	}

	clone := filepath.Join(dir, "clone")
	cloneMain(t, bare, clone)

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(clone, filepath.FromSlash(f.Path))) //nolint:gosec // G304: test clone dir
		if err != nil {
			t.Fatalf("read %q: %v", f.Path, err)
		}
		if string(data) != string(f.Data) {
			t.Errorf("%q = %q, want %q", f.Path, data, f.Data)
		}
	}
}

func TestExportFilesWithBranch_pruneRemovesStaleFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := initBareRepo(t, dir)
	cfg := Config{Endpoint: "file://" + bare, Prune: true}

	first := []FileEntry{
		{Path: "prod-west/team-a/deployment/api.yaml", Data: []byte("name: api\n")},
		{Path: "prod-west/team-a/deployment/web.yaml", Data: []byte("name: web\n")},
	}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, first, nil, CommitContext{Checksum: "c1"}); err != nil {
		t.Fatalf("first export: %v", err)
	}

	// web is removed from the snapshot; api content changes.
	second := []FileEntry{
		{Path: "prod-west/team-a/deployment/api.yaml", Data: []byte("name: api\nimage: nginx\n")},
	}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, second, nil, CommitContext{Checksum: "c2"}); err != nil {
		t.Fatalf("second export: %v", err)
	}

	clone := filepath.Join(dir, "clone")
	cloneMain(t, bare, clone)

	if _, err := os.Stat(filepath.Join(clone, "prod-west/team-a/deployment/api.yaml")); err != nil {
		t.Errorf("api.yaml should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clone, "prod-west/team-a/deployment/web.yaml")); !os.IsNotExist(err) {
		t.Errorf("web.yaml should be pruned, stat err = %v", err)
	}
}

func TestManagedDirs(t *testing.T) {
	t.Parallel()

	got := managedDirs([]string{"a/b/c.yaml", "a/b/d.yaml", "x/y.yaml", "root.yaml"})
	want := map[string]bool{"a/b": true, "x": true}
	if len(got) != len(want) {
		t.Fatalf("managedDirs = %v, want keys %v", got, want)
	}
	for _, d := range got {
		if !want[d] {
			t.Errorf("unexpected managed dir %q", d)
		}
	}
}
