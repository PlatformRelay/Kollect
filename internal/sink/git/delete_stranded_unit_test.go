// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// reviewF1/R1 lock (go-git engine): a deletion commit stranded on a
// pre-existing push branch strictly ahead of the remote push tip is a genuine
// fast-forward and must be delivered.
func TestDeliverRemoteStrandedDeletion_DeliversGenuineFastForward(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("base\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "feature base")
	runGitC(t, work, "push", "origin", "feature")

	// Stranded deletion commit: local feature is now strictly ahead of
	// origin/feature (the push never reached the remote).
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("removed\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", deleteCommitMessage)

	localHead := gitOutput(t, work, "rev-parse", "HEAD")

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if err := deliverRemoteStrandedDeletion(t.Context(), repo, Config{}.withDefaults(), nil, req, wt, false, true); err != nil {
		t.Fatalf("deliverRemoteStrandedDeletion: %v", err)
	}

	remoteTip := remoteSHAFromLsRemote(gitOutput(t, "", "ls-remote", remote, "refs/heads/feature"))
	if remoteTip != localHead {
		t.Fatalf("remote feature tip = %q, want the stranded commit %q (deletion was orphaned)", remoteTip, localHead)
	}
}

// reviewR1 lock: a pre-existing local push branch poisoned by a foreign export
// (a remote tip that is not an ancestor of HEAD) must be a no-op: the deletion
// must never force unrelated remote state.
func TestDeliverRemoteStrandedDeletion_DivergedRemoteIsNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("base\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "feature base")
	runGitC(t, work, "push", "origin", "feature")
	remoteTip := remoteSHAFromLsRemote(gitOutput(t, "", "ls-remote", remote, "refs/heads/feature"))

	// Rewrite local feature onto a divergent history: the remote tip is now
	// unrelated (not an ancestor) and must not be overwritten.
	runGitC(t, work, "reset", "--hard", "main")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("diverged\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "diverged")

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if err := deliverRemoteStrandedDeletion(t.Context(), repo, Config{}.withDefaults(), nil, req, wt, false, true); err != nil {
		t.Fatalf("deliverRemoteStrandedDeletion: %v", err)
	}

	if after := remoteSHAFromLsRemote(gitOutput(t, "", "ls-remote", remote, "refs/heads/feature")); after != remoteTip {
		t.Fatalf("diverged remote tip was overwritten: got %q, want %q", after, remoteTip)
	}
}

// reviewF1 lock: without a pre-existing local push branch there is no stranded
// deletion commit, and the checkout's synthesized pointer must never be pushed.
func TestDeliverRemoteStrandedDeletion_NoPreExistingBranchIsNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if err := deliverRemoteStrandedDeletion(t.Context(), repo, Config{}.withDefaults(), nil, req, wt, false, false); err != nil {
		t.Fatalf("deliverRemoteStrandedDeletion: %v", err)
	}

	if tip := gitOutput(t, "", "ls-remote", remote, "refs/heads/feature"); tip != "" {
		t.Fatalf("no-op delete created remote feature at %q", tip)
	}
}

// reviewF1 lock: a pre-existing local branch whose HEAD still equals the clone
// tip (the remote push branch is absent) is the nothing-matched no-op, not a
// stranded deletion to deliver.
func TestDeliverRemoteStrandedDeletion_CloneTipNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if err := deliverRemoteStrandedDeletion(t.Context(), repo, Config{}.withDefaults(), nil, req, wt, false, true); err != nil {
		t.Fatalf("deliverRemoteStrandedDeletion: %v", err)
	}

	if tip := gitOutput(t, "", "ls-remote", remote, "refs/heads/feature"); tip != "" {
		t.Fatalf("clone-tip no-op created remote feature at %q", tip)
	}
}

// reviewF1/R1 lock (CLI engine): the genuine stranded deletion — a local push
// tip strictly ahead of the remote push tip — is delivered, while a missing or
// diverged tip is a no-op.
func TestCLIStrandedDeliveryDue_GenuineFastForwardDelivered(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("base\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "feature base")
	runGitC(t, work, "push", "origin", "feature")

	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("removed\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", deleteCommitMessage)
	localHead := gitOutput(t, work, "rev-parse", "HEAD")

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	deliver, err := cliStrandedDeliveryDue(t.Context(), work, req, nil, localHead, true)
	if err != nil {
		t.Fatalf("cliStrandedDeliveryDue: %v", err)
	}
	if !deliver {
		t.Fatal("stranded deletion strictly ahead of the remote tip must be delivered")
	}
}

func TestCLIStrandedDeliveryDue_NoopsForForeignOrMissingTips(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("base\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "feature base")
	runGitC(t, work, "push", "origin", "feature")
	localHead := gitOutput(t, work, "rev-parse", "HEAD")

	req := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}

	// No pre-existing local tip: the synthesized pointer must never be pushed.
	if deliver, err := cliStrandedDeliveryDue(t.Context(), work, req, nil, "", false); err != nil || deliver {
		t.Fatalf("missing pre-existing tip: deliver=%v err=%v, want false/nil", deliver, err)
	}

	// HEAD no longer matches the captured pre-existing tip: no stranded commit.
	if deliver, err := cliStrandedDeliveryDue(t.Context(), work, req, nil, strings.Repeat("0", 40), true); err != nil || deliver {
		t.Fatalf("stale captured tip: deliver=%v err=%v, want false/nil", deliver, err)
	}

	// Remote tip diverged from HEAD: a pure fast-forward is impossible.
	runGitC(t, work, "reset", "--hard", "main")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("diverged\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "diverged")
	divergedHead := gitOutput(t, work, "rev-parse", "HEAD")
	if deliver, err := cliStrandedDeliveryDue(t.Context(), work, req, nil, divergedHead, true); err != nil || deliver {
		t.Fatalf("diverged remote: deliver=%v err=%v, want false/nil", deliver, err)
	}

	// The remote feature tip must be untouched throughout.
	if tip := remoteSHAFromLsRemote(gitOutput(t, "", "ls-remote", remote, "refs/heads/feature")); tip != localHead {
		t.Fatalf("remote feature tip = %q, want unchanged %q", tip, localHead)
	}
}

func TestDeleteExport_WrapperUsesObjectPathCommitContext(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	backend := &Backend{cfg: Config{Endpoint: "file://" + remote}.withDefaults()}

	deleted, err := backend.DeleteExport(t.Context(), []string{"inventory/team-a/never-exported.json"})
	if err != nil || len(deleted) != 0 {
		t.Fatalf("DeleteExport no-op = %v/%v, want empty/nil", deleted, err)
	}
}

func TestRemoteSHAFromLsRemote(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                         "",
		"abc\trefs/heads/main":     "abc",
		"abc refs/heads/main":      "abc",
		"abc":                      "abc",
		"  abc\trefs/heads/main  ": "abc",
	}
	for in, want := range cases {
		if got := remoteSHAFromLsRemote(in); got != want {
			t.Errorf("remoteSHAFromLsRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateDeletePaths_EdgeCases(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/kollect-delete-paths.git"}.withDefaults()

	// Blank paths are skipped; when nothing remains there is no work and no
	// resolved request.
	req, paths, err := validateDeletePaths(cfg, []string{"", "   "}, nil)
	if err != nil || len(paths) != 0 || req.cloneURL != "" {
		t.Fatalf("blank paths = %+v/%v/%v, want zero request and no error", req, paths, err)
	}

	// Duplicate paths collapse to one candidate.
	_, paths, err = validateDeletePaths(cfg, []string{"inventory/team-a/inv.json", "inventory/team-a/inv.json"}, nil)
	if err != nil || len(paths) != 1 {
		t.Fatalf("duplicate paths = %v/%v, want one candidate", paths, err)
	}

	// Traversal is rejected before any remote work.
	if _, _, err := validateDeletePaths(cfg, []string{"../evil.json"}, nil); err == nil {
		t.Fatal("traversal path must be rejected")
	}

	// A malformed endpoint cannot be resolved to a clone URL.
	if _, _, err := validateDeletePaths(Config{Endpoint: "://bad"}.withDefaults(), []string{"inventory/team-a/inv.json"}, nil); err == nil {
		t.Fatal("malformed endpoint must be rejected")
	}

	// An invalid branch ref is rejected.
	if _, _, err := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, &BranchSpec{CloneBranch: "; rm -rf /"}); err == nil {
		t.Fatal("invalid clone branch must be rejected")
	}
}

func TestRemoteTipFastForwardable(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "feature.txt"), []byte("base\n"))
	runGitC(t, work, "add", "feature.txt")
	runGitC(t, work, "commit", "-m", "feature base")

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("CommitObject(head): %v", err)
	}
	parent := headCommit.ParentHashes[0]

	if !remoteTipFastForwardable(repo, parent, head.Hash()) {
		t.Errorf("parent must be fast-forwardable to its child")
	}
	if !remoteTipFastForwardable(repo, head.Hash(), head.Hash()) {
		t.Errorf("a tip equal to HEAD must be fast-forwardable")
	}
	if remoteTipFastForwardable(repo, plumbing.ZeroHash, head.Hash()) {
		t.Errorf("an unknown remote tip must not be fast-forwardable")
	}
}

func TestGitHeadHashAndAncestry(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	head := gitOutput(t, work, "rev-parse", "HEAD")
	got, err := gitHeadHash(t.Context(), work, nil)
	if err != nil || got != head {
		t.Fatalf("gitHeadHash = %q/%v, want %q", got, err, head)
	}

	if ancestor, err := gitIsAncestorOfHead(t.Context(), work, head, nil); err != nil || !ancestor {
		t.Fatalf("HEAD must be an ancestor of itself: %v/%v", ancestor, err)
	}
	if ancestor, err := gitIsAncestorOfHead(t.Context(), work, strings.Repeat("0", 40), nil); err != nil || ancestor {
		t.Fatalf("unknown commit must not be an ancestor: %v/%v", ancestor, err)
	}
	if ancestor, err := gitIsAncestorOfHead(t.Context(), work, "not-a-hash", nil); err != nil || ancestor {
		t.Fatalf("non-hash must be rejected without error: %v/%v", ancestor, err)
	}
}

func TestIsCommitHash(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		strings.Repeat("a", 40): true,
		strings.Repeat("0", 40): true,
		strings.Repeat("A", 40): true,
		strings.Repeat("a", 39): false,
		strings.Repeat("a", 41): false,
		strings.Repeat("g", 40): false,
		"":                      false,
	}
	for hash, want := range cases {
		if got := isCommitHash(hash); got != want {
			t.Errorf("isCommitHash(%q) = %v, want %v", hash, got, want)
		}
	}
}
