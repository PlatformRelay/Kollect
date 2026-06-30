// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// exportRemote is the go-git-native push path (as opposed to exportViaCLI). The high-level
// ExportFilesWithBranch entry point always routes file:// remotes through the CLI path, so
// exportRemote is exercised here directly with a local bare repo to cover the in-memory
// worktree write/commit/push flow without requiring a real network-reachable git host.
func TestExportRemote_PushesFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	files := []FileEntry{{Path: "inventory/latest.json", Data: []byte(`{"hello":"world"}`)}}
	req, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
		t.Fatalf("exportRemote() error = %v", exportErr)
	}

	verify := t.TempDir()
	if out, cloneErr := exec.Command("git", "clone", "--branch", "main", "--single-branch", remote, verify).CombinedOutput(); cloneErr != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone verify: %s: %v", out, cloneErr)
	}

	data, err := os.ReadFile(filepath.Join(verify, "inventory", "latest.json")) //nolint:gosec // G304: test fixture
	if err != nil {
		t.Fatalf("read inventory/latest.json: %v", err)
	}
	if string(data) != `{"hello":"world"}` {
		t.Fatalf("payload = %q", data)
	}
}

func TestExportRemote_NoChangesIsNoop(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	files := []FileEntry{{Path: "README.md", Data: []byte("seed\n")}}
	req, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
		t.Fatalf("exportRemote() error = %v, want clean-status no-op", exportErr)
	}
}

func TestCloneOrInit_emptyRemoteFallsBackToInit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "empty.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", bare).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	workdir := filepath.Join(dir, "work")
	repo, emptyRemote, err := cloneOrInit(t.Context(), workdir, "file://"+bare, "main", nil, Config{CloneDepth: 1})
	if err != nil {
		t.Fatalf("cloneOrInit() error = %v", err)
	}
	if !emptyRemote {
		t.Fatal("cloneOrInit() emptyRemote = false, want true for an empty bare repo")
	}
	if repo == nil {
		t.Fatal("cloneOrInit() repo = nil")
	}

	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		t.Fatalf(".git missing after init fallback: %v", err)
	}
}

func TestCloneOrInit_nonEmptyRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	workdir := filepath.Join(t.TempDir(), "work")

	repo, emptyRemote, err := cloneOrInit(t.Context(), workdir, "file://"+remote, "main", nil, Config{CloneDepth: 1})
	if err != nil {
		t.Fatalf("cloneOrInit() error = %v", err)
	}
	if emptyRemote {
		t.Fatal("cloneOrInit() emptyRemote = true, want false for seeded repo")
	}
	if repo == nil {
		t.Fatal("cloneOrInit() repo = nil")
	}
	if _, err := repo.Head(); err != nil {
		t.Fatalf("repo.Head() error = %v, want resolvable HEAD", err)
	}
}
