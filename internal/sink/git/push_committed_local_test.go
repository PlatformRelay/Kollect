// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// goGitCloneMain clones a bare remote at file://remote onto a fresh worktree via go-git and returns
// the repo, worktree, and current HEAD hash.
func goGitCloneMain(t *testing.T, remote string) (*git.Repository, *git.Worktree, plumbing.Hash) {
	t.Helper()

	repo, err := git.PlainCloneContext(t.Context(), filepath.Join(t.TempDir(), "clone"), false, &git.CloneOptions{
		URL:           "file://" + remote,
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
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	return repo, wt, head.Hash()
}

// advanceRemoteMain pushes a new commit onto the bare remote's main branch via the git CLI, so the
// remote diverges from an already-cloned worktree.
func advanceRemoteMain(t *testing.T, remote string, filename, content string) {
	t.Helper()

	work := t.TempDir()
	runGit(t, "clone", "--branch", "main", "--single-branch", "file://"+remote, work)
	runGitC(t, work, "config", "user.name", "Remote Advancer")
	runGitC(t, work, "config", "user.email", "advancer@example.com")
	mustWriteFile(t, filepath.Join(work, filename), []byte(content))
	runGitC(t, work, "add", ".")
	runGitC(t, work, "commit", "-m", "advance remote")
	runGitC(t, work, "push", "origin", "main")
}

// localCommit writes a file into the worktree and commits it, returning the new commit hash.
func localCommit(t *testing.T, wt *git.Worktree, dir, rel, content string) plumbing.Hash {
	t.Helper()

	mustWriteFile(t, filepath.Join(dir, rel), []byte(content))
	if _, err := wt.Add(rel); err != nil {
		t.Fatalf("Add(%q): %v", rel, err)
	}
	hash, err := wt.Commit("local change", &git.CommitOptions{
		Author: &object.Signature{Name: "Local", Email: "local@example.com"},
	})
	if err != nil {
		t.Fatalf("Commit(): %v", err)
	}
	return hash
}

// TestPushCommitted_NonFastForwardSyncFastForwards covers the Commit-policy non-fast-forward recovery
// branch of pushCommitted: the local worktree is behind after the remote advanced, so the initial
// push is rejected non-fast-forward, syncRemoteBeforePush fast-forwards the worktree, and the retry
// push succeeds (returns nil). This is the success-after-sync path.
func TestPushCommitted_NonFastForwardSyncFastForwards(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	repo, wt, base := goGitCloneMain(t, remote)

	// Advance the remote AFTER cloning so the local worktree is strictly behind (fast-forwardable).
	advanceRemoteMain(t, remote, "remote-added.txt", "from remote\n")

	err := pushCommitted(
		t.Context(), repo, Config{PushPolicy: PushPolicyCommit}, nil,
		"file://"+remote, "main", false, base, wt,
	)
	if err != nil {
		t.Fatalf("pushCommitted() error = %v, want nil after fast-forward sync", err)
	}
}

// TestPushCommitted_NonReconcilingPolicySurfacesNonFastForward asserts the contract that a
// non-reconciling push policy surfaces a non-fast-forward rejection wrapped and classified, rather
// than swallowing it. The default PushPolicyCommit reconciles a non-ff by force-fetching the remote
// tip (superseding the local commit), so the raw non-ff error only reaches the caller under a
// non-Commit policy. Here the local worktree holds a divergent commit and the remote advanced
// independently, so the (non-forced) push is rejected non-fast-forward.
func TestPushCommitted_NonReconcilingPolicySurfacesNonFastForward(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	repo, wt, _ := goGitCloneMain(t, remote)

	local := localCommit(t, wt, wt.Filesystem.Root(), "local-only.txt", "local change\n")
	advanceRemoteMain(t, remote, "remote-only.txt", "remote change\n")

	// PushPolicy "" is a non-reconciling, non-forcing policy: no sync branch, no force refspec.
	err := pushCommitted(
		t.Context(), repo, Config{PushPolicy: ""}, nil,
		"file://"+remote, "main", false, local, wt,
	)
	t.Logf("non-reconciling pushCommitted error = %v", err)
	if err == nil {
		t.Fatal("pushCommitted() error = nil, want a wrapped non-fast-forward rejection")
	}
	if !isNonFastForwardError(err) {
		t.Fatalf("pushCommitted() error = %v, want a non-fast-forward classified error", err)
	}
	if !strings.Contains(err.Error(), "git push") {
		t.Fatalf("pushCommitted() error = %v, want git push wrapper", err)
	}
}

// TestPushCommitted_PushErrorIsWrapped covers the terminal push-error branch: a push to a
// nonexistent remote URL fails for a non-fast-forward-unrelated reason and pushCommitted wraps it as
// "git push".
func TestPushCommitted_PushErrorIsWrapped(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	repo, wt, _ := goGitCloneMain(t, remote)
	local := localCommit(t, wt, wt.Filesystem.Root(), "local-only.txt", "local change\n")

	missing := "file://" + filepath.Join(t.TempDir(), "nonexistent.git")
	err := pushCommitted(
		t.Context(), repo, Config{PushPolicy: PushPolicyCommit}, nil,
		missing, "main", false, local, wt,
	)
	t.Logf("push-error pushCommitted error = %v", err)
	if err == nil {
		t.Fatal("pushCommitted() error = nil, want wrapped push failure")
	}
	if !strings.Contains(err.Error(), "git push") {
		t.Fatalf("pushCommitted() error = %v, want git push wrapper", err)
	}
}

// TestStageChanges_NoPruneAddErrorIsWrapped covers the non-prune wt.Add error branch: staging a path
// that does not exist in the worktree makes go-git's Add fail and stageChanges wraps it as "git add".
func TestStageChanges_NoPruneAddErrorIsWrapped(t *testing.T) {
	t.Parallel()

	_, wt, _ := initRepoWithSeedCommit(t)

	err := stageChanges(wt, []string{"inventory/does-not-exist.json"}, false)
	if err == nil {
		t.Fatal("stageChanges() error = nil, want wrapped git add error for missing path")
	}
	if !strings.Contains(err.Error(), "git add") {
		t.Fatalf("stageChanges() error = %v, want git add wrapper", err)
	}
}
