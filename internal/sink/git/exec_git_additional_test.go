// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitFetchShallowAndPullRebase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remoteDir := createBareRemoteWithMainCommit(t)
	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "clone", "--branch", "main", "--single-branch", remoteDir, cloneDir)

	if err := gitFetchShallow(t.Context(), cloneDir, "main", 1, nil); err != nil {
		t.Fatalf("gitFetchShallow() error = %v", err)
	}
	if err := gitPullRebase(t.Context(), cloneDir, "main", nil); err != nil {
		t.Fatalf("gitPullRebase() error = %v", err)
	}
}

func TestGitPullRebase_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	if err := gitPullRebase(t.Context(), t.TempDir(), "--upload-pack=evil", nil); err == nil {
		t.Fatal("expected error for flag-like branch")
	}
}

func TestGitStatusPorcelain_rejectsInvalidWorkdir(t *testing.T) {
	t.Parallel()

	if _, err := gitStatusPorcelain(t.Context(), "bad\x00dir", nil); err == nil {
		t.Fatal("expected error for invalid workdir")
	}
}

func TestGitCommit_rejectsInvalidAuthorName(t *testing.T) {
	t.Parallel()

	err := gitCommit(t.Context(), t.TempDir(), "-bad-name", "bot@example.com", renderedCommit{Subject: "msg"}, nil)
	if err == nil {
		t.Fatal("expected error for flag-like author name")
	}
}

func TestGitCommit_rejectsInvalidAuthorEmail(t *testing.T) {
	t.Parallel()

	err := gitCommit(t.Context(), t.TempDir(), "Bot", "-bad-email", renderedCommit{Subject: "msg"}, nil)
	if err == nil {
		t.Fatal("expected error for flag-like author email")
	}
}

func TestGitCommit_rejectsInvalidMessage(t *testing.T) {
	t.Parallel()

	err := gitCommit(t.Context(), t.TempDir(), "Bot", "bot@example.com", renderedCommit{Subject: "bad\x00msg"}, nil)
	if err == nil {
		t.Fatal("expected error for null byte in commit message")
	}
}
