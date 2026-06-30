// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsCLIEmptyRemote(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "remote repository is empty", output: "fatal: remote repository is empty", err: errors.New("exit status 128"), want: true},
		{name: "couldn't find remote ref", err: errors.New("fatal: couldn't find remote ref main"), want: true},
		{name: "reference not found", err: errors.New("reference not found"), want: true},
		{name: "repository not found", err: errors.New("repository not found"), want: true},
		{name: "remote branch not found", err: errors.New("remote branch main not found"), want: true},
		{name: "unrelated failure", err: errors.New("permission denied"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isCLIEmptyRemote(tc.output, tc.err); got != tc.want {
				t.Fatalf("isCLIEmptyRemote(%q, %v) = %v, want %v", tc.output, tc.err, got, tc.want)
			}
		})
	}
}

func TestGitStatusClean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	runGit(t, "init", "-b", "main", dir)
	runGitC(t, dir, "config", "user.name", "Kollect Tests")
	runGitC(t, dir, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(dir, "README.md"), []byte("seed\n"))
	runGitC(t, dir, "add", ".")
	runGitC(t, dir, "commit", "-m", "seed")

	clean, err := gitStatusClean(t.Context(), dir, nil)
	if err != nil {
		t.Fatalf("gitStatusClean() error = %v", err)
	}
	if !clean {
		t.Fatal("expected clean status after commit")
	}

	mustWriteFile(t, filepath.Join(dir, "dirty.txt"), []byte("untracked\n"))

	clean, err = gitStatusClean(t.Context(), dir, nil)
	if err != nil {
		t.Fatalf("gitStatusClean() error = %v", err)
	}
	if clean {
		t.Fatal("expected dirty status with untracked file present")
	}
}

func TestGitPushOriginWithRecovery_shortCircuitsOnForce(t *testing.T) {
	t.Parallel()

	err := gitPushOriginWithRecovery(t.Context(), t.TempDir(), true, "--upload-pack=evil", Config{PushPolicy: PushPolicyCommit}, nil)
	if err == nil {
		t.Fatal("expected validation error from gitPushOrigin to propagate when force=true")
	}
}

func TestGitPushOriginWithRecovery_shortCircuitsWhenNotCommitPolicy(t *testing.T) {
	t.Parallel()

	err := gitPushOriginWithRecovery(t.Context(), t.TempDir(), false, "--upload-pack=evil", Config{PushPolicy: PushPolicyForcePush}, nil)
	if err == nil {
		t.Fatal("expected validation error from gitPushOrigin to propagate when push policy is not Commit")
	}
}

func TestGitPushOriginWithRecovery_nonFastForwardConflictRetriesAfterRebase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)

	cloneA := filepath.Join(t.TempDir(), "clone-a")
	runGit(t, "clone", "--branch", "main", "--single-branch", remote, cloneA)
	runGitC(t, cloneA, "config", "user.name", "Kollect Tests")
	runGitC(t, cloneA, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(cloneA, "from-a.txt"), []byte("a\n"))
	runGitC(t, cloneA, "add", ".")
	runGitC(t, cloneA, "commit", "-m", "from clone a")

	cloneB := filepath.Join(t.TempDir(), "clone-b")
	runGit(t, "clone", "--branch", "main", "--single-branch", remote, cloneB)
	runGitC(t, cloneB, "config", "user.name", "Kollect Tests")
	runGitC(t, cloneB, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(cloneB, "from-b.txt"), []byte("b\n"))
	runGitC(t, cloneB, "add", ".")
	runGitC(t, cloneB, "commit", "-m", "from clone b")
	runGitC(t, cloneB, "push", "origin", "main")

	cfg := Config{PushPolicy: PushPolicyCommit}
	if err := gitPushOriginWithRecovery(t.Context(), cloneA, false, "main", cfg, nil); err != nil {
		t.Fatalf("gitPushOriginWithRecovery() error = %v, want rebase-then-retry to succeed", err)
	}

	verify := filepath.Join(t.TempDir(), "verify")
	runGit(t, "clone", "--branch", "main", "--single-branch", remote, verify)
	if _, statErr := os.Stat(filepath.Join(verify, "from-a.txt")); statErr != nil {
		t.Fatalf("expected from-a.txt to be present after rebase-retry push: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(verify, "from-b.txt")); statErr != nil {
		t.Fatalf("expected from-b.txt to remain present after rebase-retry push: %v", statErr)
	}
}
