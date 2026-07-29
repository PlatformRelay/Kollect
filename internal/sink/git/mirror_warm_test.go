// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestOpenOrWarmMirror_ColdThenWarm(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remoteDir := createBareRemoteWithMainCommit(t)
	mirrorDir := filepath.Join(t.TempDir(), "mirror")
	cloneURL := "file://" + remoteDir

	repo, emptyRemote, err := openOrWarmMirror(
		t.Context(),
		mirrorDir,
		cloneURL,
		"main",
		1,
		nil,
		Config{CloneDepth: 1},
	)
	if err != nil {
		t.Fatalf("openOrWarmMirror(cold) error = %v", err)
	}
	if repo == nil {
		t.Fatal("openOrWarmMirror(cold) repo = nil")
	}
	if emptyRemote {
		t.Fatal("openOrWarmMirror(cold) emptyRemote = true, want false")
	}
	if !mirrorWarm(mirrorDir) {
		t.Fatal("mirror should be warm after clone")
	}

	repo, emptyRemote, err = openOrWarmMirror(
		t.Context(),
		mirrorDir,
		cloneURL,
		"main",
		1,
		nil,
		Config{CloneDepth: 1},
	)
	if err != nil {
		t.Fatalf("openOrWarmMirror(warm) error = %v", err)
	}
	if repo == nil {
		t.Fatal("openOrWarmMirror(warm) repo = nil")
	}
	if emptyRemote {
		t.Fatal("openOrWarmMirror(warm) emptyRemote = true, want false")
	}
}

func TestOpenOrWarmMirror_WarmMirrorOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatalf("MkdirAll(.git): %v", err)
	}

	_, _, err := openOrWarmMirror(t.Context(), dir, "file:///tmp/remote.git", "main", 1, nil, Config{})
	if err == nil {
		t.Fatal("openOrWarmMirror() error = nil, want open mirror error")
	}
}

func TestCheckoutMirrorBranch_NewAndExistingBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	runGit(t, "init", "-b", "main", repoDir)
	runGitC(t, repoDir, "config", "user.name", "Kollect Tests")
	runGitC(t, repoDir, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(repoDir, "README.md"), []byte("seed\n"))
	runGitC(t, repoDir, "add", ".")
	runGitC(t, repoDir, "commit", "-m", "seed")

	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("PlainOpen() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	if err := checkoutMirrorBranch(wt, "feature"); err != nil {
		t.Fatalf("checkoutMirrorBranch(new) error = %v", err)
	}
	if err := checkoutMirrorBranch(wt, "feature"); err != nil {
		t.Fatalf("checkoutMirrorBranch(existing) error = %v", err)
	}
}
