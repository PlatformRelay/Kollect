// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestSyncRemoteBeforePush_RemoteMissing(t *testing.T) {
	t.Parallel()

	repo, err := git.PlainInit(t.TempDir(), false)
	if err != nil {
		t.Fatalf("PlainInit() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	err = syncRemoteBeforePush(t.Context(), repo, wt, nil, "file:///tmp/remote.git", "main", Config{})
	if err == nil || !strings.Contains(err.Error(), "remote origin") {
		t.Fatalf("syncRemoteBeforePush() error = %v, want missing origin error", err)
	}
}

func TestSyncRemoteBeforePush_UpToDateRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remoteDir := createBareRemoteWithMainCommit(t)
	cloneURL := "file://" + remoteDir

	repo, err := git.PlainCloneContext(t.Context(), filepath.Join(t.TempDir(), "clone"), false, &git.CloneOptions{
		URL:           cloneURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("PlainCloneContext() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	if err := syncRemoteBeforePush(t.Context(), repo, wt, nil, cloneURL, "main", Config{}); err != nil {
		t.Fatalf("syncRemoteBeforePush() error = %v", err)
	}
}

func TestSyncRemoteBeforePush_FetchErrorIsWrapped(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remoteDir := createBareRemoteWithMainCommit(t)
	cloneURL := "file://" + remoteDir

	repo, err := git.PlainCloneContext(t.Context(), filepath.Join(t.TempDir(), "clone"), false, &git.CloneOptions{
		URL:           cloneURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("PlainCloneContext() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	err = syncRemoteBeforePush(t.Context(), repo, wt, nil, "ssh://invalid host", "main", Config{})
	if err == nil || !strings.Contains(err.Error(), "git fetch before push") {
		t.Fatalf("syncRemoteBeforePush() error = %v, want wrapped fetch error", err)
	}
}

func createBareRemoteWithMainCommit(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")

	runGit(t, "init", "-b", "main", work)
	runGitC(t, work, "config", "user.name", "Kollect Tests")
	runGitC(t, work, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(work, "README.md"), []byte("seed\n"))
	runGitC(t, work, "add", ".")
	runGitC(t, work, "commit", "-m", "seed")
	runGit(t, "init", "--bare", "-b", "main", remote)
	runGitC(t, work, "remote", "add", "origin", remote)
	runGitC(t, work, "push", "-u", "origin", "main")

	return remote
}

func runGit(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...) //nolint:gosec // test helper executes static git fixture commands
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func runGitC(t *testing.T, cwd string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...) //nolint:gosec // test helper executes static git fixture commands
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %v: %v (%s)", cwd, args, err, out)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
