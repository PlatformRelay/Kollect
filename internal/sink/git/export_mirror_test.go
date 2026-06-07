// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExportFingerprintSkipSameChecksum(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	mirrorRoot := filepath.Join(dir, "mirrors")
	t.Setenv(envMirrorDir, mirrorRoot)

	bare := filepath.Join(dir, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("init bare: %s: %v", out, err)
	}

	endpoint := "file://" + bare
	cfg := Config{Endpoint: endpoint}
	payload := []byte(`{"items":[{"uid":"u1"}]}`)
	commitCtx := CommitContext{Checksum: "sha256:deadbeef"}

	if err := ExportWithBranch(t.Context(), cfg, Auth{}, payload, "inventory/test.json", nil, commitCtx); err != nil {
		t.Fatalf("first export: %v", err)
	}

	cloneDir := filepath.Join(dir, "clone")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", endpoint, cloneDir).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone: %s: %v", out, err)
	}

	headBefore, err := exec.Command("git", "-C", cloneDir, "rev-parse", "HEAD").Output() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}

	if exportErr := ExportWithBranch(t.Context(), cfg, Auth{}, payload, "inventory/test.json", nil, commitCtx); exportErr != nil {
		t.Fatalf("second export (fingerprint skip): %v", exportErr)
	}

	headAfter, err := exec.Command("git", "-C", cloneDir, "rev-parse", "HEAD").Output() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("rev-parse after skip: %v", err)
	}

	if string(headBefore) != string(headAfter) {
		t.Fatalf("expected no new commit on fingerprint skip: before=%s after=%s", headBefore, headAfter)
	}
}

func TestMirrorDirForAndWarm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envMirrorDir, dir)

	mirrorDir, err := mirrorDirFor("https://example.com/r.git", "main")
	if err != nil {
		t.Fatal(err)
	}

	if mirrorWarm(mirrorDir) {
		t.Fatal("expected cold mirror")
	}

	if err := os.MkdirAll(filepath.Join(mirrorDir, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}

	if !mirrorWarm(mirrorDir) {
		t.Fatal("expected warm mirror after .git created")
	}
}

func TestPrepareMirrorWorkdir_fileRemoteUsesTemp(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("init bare: %s: %v", out, err)
	}

	workdir, err := prepareMirrorWorkdir(t.Context(), Config{Endpoint: "file://" + bare}, Auth{}, "file://"+bare, "main")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(workdir) }()

	if workdir == "" || workdir == filepath.Join(os.TempDir(), "kollect-git-mirrors") {
		t.Fatalf("unexpected workdir %q", workdir)
	}
}
