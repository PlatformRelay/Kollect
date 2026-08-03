// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportFilesWithBranch_EmptyFilesGuard covers the no-files guard at the top of
// ExportFilesWithBranch.
func TestExportFilesWithBranch_EmptyFilesGuard(t *testing.T) {
	t.Parallel()

	err := ExportFilesWithBranch(t.Context(), Config{Endpoint: "file:///tmp/x.git"}, Auth{}, nil, nil, CommitContext{})
	if err == nil || !strings.Contains(err.Error(), "no files to write") {
		t.Fatalf("ExportFilesWithBranch() error = %v, want no-files guard", err)
	}
}

// TestExportRemote_InsecureSkipVerifyFeatureBranch drives exportRemote (go-git engine) against a bare
// local repo with TLS.InsecureSkipVerify set and a push branch distinct from the clone branch,
// covering the insecure-SSH propagation and the feature-branch checkout branch. It then verifies the
// file landed on the feature branch of the remote.
func TestExportRemote_InsecureSkipVerifyFeatureBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()
	cfg.TLS.InsecureSkipVerify = true

	files := []FileEntry{{Path: "inventory/feature.json", Data: []byte(`{"branch":"feature"}`)}}
	req, validated, err := validateExportFiles(cfg, files, &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"})
	if err != nil {
		t.Fatalf("validateExportFiles: %v", err)
	}
	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
		t.Fatalf("exportRemote() error = %v", exportErr)
	}

	verify := t.TempDir()
	if out, cloneErr := exec.Command("git", "clone", "--branch", "kollect/team-a/inv", "--single-branch", remote, verify).CombinedOutput(); cloneErr != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone feature branch: %s: %v", out, cloneErr)
	}
	if _, statErr := os.Stat(filepath.Join(verify, "inventory", "feature.json")); statErr != nil {
		t.Fatalf("expected inventory/feature.json on feature branch: %v", statErr)
	}
}

// TestExportRemote_PrepareFailureWrapsError covers the mirror-preparation error path of exportRemote:
// a file:// clone URL pointing at a nonexistent bare repo cannot be warmed, so exportRemote surfaces a
// wrapped error rather than panicking.
func TestExportRemote_PrepareFailureWrapsError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	missing := filepath.Join(t.TempDir(), "nonexistent.git")
	cfg := Config{Endpoint: "file://" + missing}.withDefaults()

	files := []FileEntry{{Path: "inventory/latest.json", Data: []byte(`{"x":1}`)}}
	req, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		t.Fatalf("validateExportFiles: %v", err)
	}
	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr == nil {
		t.Fatal("exportRemote() error = nil, want wrapped mirror-preparation failure")
	}
}

// TestExportFilesWithBranch_CLIPathClassifiesError drives the CLI-engine export path (file:// remotes
// route through the git CLI) against a nonexistent bare repo, so the clone fails and the error is
// returned through ClassifyExportError rather than swallowed.
func TestExportFilesWithBranch_CLIPathClassifiesError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	missing := filepath.Join(t.TempDir(), "nonexistent.git")
	cfg := Config{Endpoint: "file://" + missing, Engine: GitEngineCLI}

	files := []FileEntry{{Path: "inventory/latest.json", Data: []byte(`{"x":1}`)}}
	err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{})
	if err == nil {
		t.Fatal("ExportFilesWithBranch() error = nil, want wrapped CLI clone failure")
	}
}
