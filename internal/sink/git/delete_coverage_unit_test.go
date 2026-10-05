// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// A deletion with no candidate paths has no work and must not touch the remote.
func TestDeleteExportWithBranch_EmptyPathsIsNoop(t *testing.T) {
	t.Parallel()

	deleted, err := DeleteExportWithBranch(t.Context(), Config{}.withDefaults(), Auth{}, nil, nil, CommitContext{})
	if err != nil || deleted != nil {
		t.Fatalf("DeleteExportWithBranch(nil) = %v/%v, want nil/nil", deleted, err)
	}
}

// A traversal candidate is rejected before any remote work.
func TestDeleteExportWithBranch_InvalidPathIsError(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/kollect-delete-invalid.git"}.withDefaults()
	if _, err := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"../evil.json"}, nil, CommitContext{}); err == nil {
		t.Fatal("traversal path must be rejected")
	}
}

// The push branch and the clone URL are validated independently of the clone
// branch, and an unsupported scheme is rejected.
func TestValidateDeletePaths_RejectsPushBranchAndScheme(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/kollect-delete-paths.git"}.withDefaults()
	if _, _, err := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, &BranchSpec{
		CloneBranch: "main",
		PushBranch:  "bad ref",
	}); err == nil {
		t.Fatal("invalid push branch must be rejected")
	}

	unsupported := Config{Endpoint: "git://example.com/repo.git"}.withDefaults()
	if _, _, err := validateDeletePaths(unsupported, []string{"inventory/team-a/inv.json"}, nil); err == nil {
		t.Fatal("unsupported clone URL scheme must be rejected")
	}
}

// A file:// remote that cannot be cloned surfaces as a deletion error (never a
// silent success): the CLI engine cannot prepare a workdir.
func TestDeleteExportWithBranch_UnclonableFileRemoteErrors(t *testing.T) {
	skipWithoutGit(t)

	cfg := Config{Endpoint: "file:///nonexistent/kollect/remote.git"}.withDefaults()
	if _, err := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/inv.json"}, nil, CommitContext{}); err == nil {
		t.Fatal("unclonable remote must surface an error, not a silent success")
	}
}

func initRepoWithCommit(t *testing.T) (*git.Repository, string) {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	mustWriteFile(t, filepath.Join(dir, "README.md"), []byte("seed\n"))
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("seed", &git.CommitOptions{
		Author: &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com"},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return repo, dir
}

// pushBranchWithoutWork must classify an invalid workdir, a dirty tree, an
// unborn HEAD, and a repo without an origin remote as "not provably empty"
// rather than crashing or reporting a false no-op.
func TestPushBranchWithoutWork_NotProvablyEmpty(t *testing.T) {
	skipWithoutGit(t)

	if _, _, err := pushBranchWithoutWork(t.Context(), "bad\x00dir", nil, "main", "feature"); err == nil {
		t.Fatal("invalid workdir must error")
	}

	_, dirtyDir := initRepoWithCommit(t)
	mustWriteFile(t, filepath.Join(dirtyDir, "dirty.txt"), []byte("dirty\n"))
	if empty, _, err := pushBranchWithoutWork(t.Context(), dirtyDir, nil, "main", "feature"); err != nil || empty {
		t.Fatalf("dirty tree = %v/%v, want false/nil", empty, err)
	}

	unborn := t.TempDir()
	if _, err := git.PlainInit(unborn, false); err != nil {
		t.Fatalf("PlainInit(unborn): %v", err)
	}
	if _, _, err := pushBranchWithoutWork(t.Context(), unborn, nil, "main", "feature"); err == nil {
		t.Fatal("unborn HEAD must error on rev-parse")
	}

	_, noOrigin := initRepoWithCommit(t)
	if _, _, err := pushBranchWithoutWork(t.Context(), noOrigin, nil, "main", "feature"); err == nil {
		t.Fatal("missing origin must error on ls-remote")
	}
}

// A clean worktree at the remote clone tip with the push branch absent is the
// provable no-op: the clone-branch ls-remote confirms HEAD equals the tip.
func TestPushBranchWithoutWork_CloneTipIsNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	empty, pushSHA, err := pushBranchWithoutWork(t.Context(), work, nil, "main", "feature")
	if err != nil {
		t.Fatalf("pushBranchWithoutWork: %v", err)
	}
	if !empty || pushSHA != "" {
		t.Fatalf("clone tip = %v/%q, want true and empty push tip", empty, pushSHA)
	}
}

// cliStrandedDeliveryDue surfaces a gitHeadHash failure and a probe failure as
// errors instead of guessing.
func TestCLIStrandedDeliveryDue_ErrorsPropagate(t *testing.T) {
	skipWithoutGit(t)

	req := exportRequest{cloneURL: "file:///tmp/x.git", cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if _, err := cliStrandedDeliveryDue(t.Context(), "bad\x00dir", req, nil, "", true); err == nil {
		t.Fatal("invalid workdir must surface a head error")
	}

	_, noOrigin := initRepoWithCommit(t)
	head := gitOutput(t, noOrigin, "rev-parse", "HEAD")
	if _, err := cliStrandedDeliveryDue(t.Context(), noOrigin, req, nil, head, true); err == nil {
		t.Fatal("missing origin must surface a probe error")
	}
}

// deliverRemoteStrandedDeletion surfaces a pushBranchSynced failure (no origin
// remote) as an error.
func TestDeliverRemoteStrandedDeletion_ProbeErrorPropagates(t *testing.T) {
	skipWithoutGit(t)

	repo, _ := initRepoWithCommit(t)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	req := exportRequest{cloneURL: "file:///tmp/x.git", cloneBranch: "main", pushBranch: "feature", objectPath: "inventory/team-a/inv.json"}
	if err := deliverRemoteStrandedDeletion(t.Context(), repo, Config{}.withDefaults(), nil, req, wt, false, true); err == nil {
		t.Fatal("missing origin must surface a probe error")
	}
}

// pushBranchSynced surfaces a missing origin and an unreachable origin as
// errors rather than misclassifying the branch as absent.
func TestPushBranchSynced_ErrorsPropagate(t *testing.T) {
	skipWithoutGit(t)

	repo, _ := initRepoWithCommit(t)
	if _, _, _, err := pushBranchSynced(t.Context(), repo, Config{}.withDefaults(), nil, "main", "feature"); err == nil {
		t.Fatal("missing origin must error")
	}

	unreachable, _ := initRepoWithCommit(t)
	if _, err := unreachable.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file:///nonexistent/kollect/missing.git"},
	}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}
	if _, _, _, err := pushBranchSynced(t.Context(), unreachable, Config{}.withDefaults(), nil, "main", "feature"); err == nil {
		t.Fatal("unreachable origin must error on list")
	}
}

// remoteTipFastForwardable refuses to treat an unreadable HEAD commit as a
// fast-forward.
func TestRemoteTipFastForwardable_UnknownHeadIsFalse(t *testing.T) {
	skipWithoutGit(t)

	repo, _ := initRepoWithCommit(t)
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("CommitObject: %v", err)
	}

	if remoteTipFastForwardable(repo, commit.Hash, plumbing.ZeroHash) {
		t.Fatal("an unreadable HEAD commit must not be fast-forwardable")
	}
}

// removeDiskCandidates tolerates a missing directory, skips subdirectories and
// non-matching entries, and removes the matching candidate.
func TestRemoveDiskCandidates_SkipsMissingAndNonMatching(t *testing.T) {
	t.Parallel()

	work := t.TempDir()

	if removed, err := removeDiskCandidates(work, []string{"inventory/team-gone/inv.json"}, nil); err != nil || removed != nil {
		t.Fatalf("missing dir = %v/%v, want nil/nil", removed, err)
	}

	// A regular file where the matcher expects a directory is a real read error,
	// not a missing dir.
	if err := os.MkdirAll(filepath.Join(work, "inventory"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	mustWriteFile(t, filepath.Join(work, "inventory", "team-b"), []byte("not a dir\n"))
	if _, err := removeDiskCandidates(work, []string{"inventory/team-b/inv.json"}, nil); err == nil {
		t.Fatal("a file where a directory is expected must error")
	}

	dir := filepath.Join(work, "inventory", "team-a")
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o750); err != nil {
		t.Fatalf("MkdirAll(subdir): %v", err)
	}
	mustWriteFile(t, filepath.Join(dir, "inv.json"), []byte("{}"))
	mustWriteFile(t, filepath.Join(dir, "inv-v2.json"), []byte("{}"))

	removed, err := removeDiskCandidates(work, []string{"inventory/team-a/inv.json"}, nil)
	if err != nil {
		t.Fatalf("removeDiskCandidates: %v", err)
	}
	if len(removed) != 1 || removed[0] != "inventory/team-a/inv.json" {
		t.Fatalf("removed = %v, want the exact candidate only", removed)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "inv-v2.json")); statErr != nil {
		t.Fatalf("non-matching sibling must survive: %v", statErr)
	}
}

// releaseInWorktree (go-git production removal) skips a missing directory, subdirectories and
// non-matching files, and stages the matching removal.
func TestReleaseInWorktree_SkipsMissingAndNonMatching(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	if removed, rmErr := releaseInWorktree(wt, Config{}, []string{"inventory/team-gone/inv.json"}); rmErr != nil || removed != nil {
		t.Fatalf("missing dir = %v/%v, want nil/nil", removed, rmErr)
	}

	if err := os.MkdirAll(filepath.Join(dir, "inventory", "team-a", "subdir"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory", "team-a", "inv.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(inv): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory", "team-a", "inv-v2.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(inv-v2): %v", err)
	}

	if _, rmErr := wt.Add("inventory/team-a/inv.json"); rmErr != nil {
		t.Fatalf("Add: %v", rmErr)
	}
	if _, rmErr := wt.Commit("seed", &git.CommitOptions{
		Author: &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com"},
	}); rmErr != nil {
		t.Fatalf("Commit: %v", rmErr)
	}

	removed, rmErr := releaseInWorktree(wt, Config{}, []string{"inventory/team-a/inv.json"})
	if rmErr != nil {
		t.Fatalf("releaseInWorktree: %v", rmErr)
	}
	if len(removed) != 1 || removed[0] != "inventory/team-a/inv.json" {
		t.Fatalf("removed = %v, want the exact candidate only", removed)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "inventory", "team-a", "inv-v2.json")); statErr != nil {
		t.Fatalf("non-matching sibling must survive: %v", statErr)
	}
}

// deleteRemote's go-git nothing-matched path routes through the stranded-tip
// handler and reports a clean no-op without touching the remote.
func TestDeleteRemote_NothingMatchedIsNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-a/never-exported.json"}, nil)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths, CommitContextFromObjectPath("inventory/team-a/never-exported.json", "prod"))
	if delErr != nil || deleted != nil {
		t.Fatalf("deleteRemote no-op = %v/%v, want nil/nil", deleted, delErr)
	}
}

// deleteRemote surfaces an unsupported clone URL scheme from the auth guard.
func TestDeleteRemote_UnsupportedSchemeErrors(t *testing.T) {
	t.Parallel()

	req := exportRequest{cloneURL: "git://example.com/repo.git", cloneBranch: "main", pushBranch: "main", objectPath: "inventory/team-a/inv.json"}
	if _, err := deleteRemote(t.Context(), Config{}.withDefaults(), Auth{}, req, []string{"inventory/team-a/inv.json"}, CommitContext{}); err == nil {
		t.Fatal("unsupported clone URL scheme must error")
	}
}

// deleteRemote surfaces an unclonable file:// remote as an open/warm error.
func TestDeleteRemote_UnclonableRemoteErrors(t *testing.T) {
	skipWithoutGit(t)

	req := exportRequest{cloneURL: "file:///nonexistent/kollect/remote.git", cloneBranch: "main", pushBranch: "main", objectPath: "inventory/team-a/inv.json"}
	if _, err := deleteRemote(t.Context(), Config{}.withDefaults(), Auth{}, req, []string{"inventory/team-a/inv.json"}, CommitContext{}); err == nil {
		t.Fatal("unclonable remote must error")
	}
}
