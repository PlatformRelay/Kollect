//go:build integration

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
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Regression locks for the review round of Kollect PR 382. All of them need the
// persistent warm-mirror topology (non-file:// remote): file:// remotes get a
// fresh temp workdir per operation (prepareMirrorWorkdir), so none of the
// defects below can reproduce there.
//
// The git-engine convergence (ADR-0803) removed the selectable CLI engine, so
// the CLI delivery machinery is reachable only for file:// remotes — always
// cold — and the CLI-engine locks against a persistent mirror (staged-add
// sweep, untracked dirt, foreign-tip no-ops, stranded-commit delivery) pin a
// surface production can no longer reach; they were removed with it. The
// go-git locks below keep pinning the surviving persistent-mirror machinery.
//
// reviewC2 regression lock: go-git in branchMR mode. A crash between the
// deletion commit and its push leaves the deletion commit stranded on the
// mirror's feature-branch ref, which openOrWarmMirror never reconciles (it
// force-fetches the CLONE branch only). The retry must DELIVER the stranded
// tip, not silently orphan it while reporting a no-op.
func TestDeleteRemote_MRMode_DeliversStrandedDeletionCommit(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")
	branchSpec := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}

	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, branchSpec, commitCtx); err != nil {
		t.Fatalf("seed export: %v", err)
	}
	targetTip := remoteBranchSHA(t, url, "main")

	// Simulate the crash between commit and push: commit the deletion into the
	// persistent mirror's feature branch, do not push.
	cloneURL, _, err := parseRemote(url)
	if err != nil {
		t.Fatalf("parseRemote: %v", err)
	}
	mirror, err := mirrorDirFor(cloneURL, "main")
	if err != nil {
		t.Fatalf("mirrorDirFor: %v", err)
	}
	repo, err := git.PlainOpen(mirror)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("mirror worktree: %v", err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branchSpec.PushBranch)}); err != nil {
		t.Fatalf("checkout feature branch: %v", err)
	}
	if _, err := wt.Remove("inventory/team-a/inv.json"); err != nil {
		t.Fatalf("stage stranded deletion: %v", err)
	}
	stranded, err := wt.Commit("chore(prod/team-a/team-a): remove inventory export (simulated crash)", &git.CommitOptions{
		Author: &object.Signature{Name: cfg.Author.Name, Email: cfg.Author.Email},
	})
	if err != nil {
		t.Fatalf("stranded commit: %v", err)
	}

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, branchSpec)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths, commitCtx)
	if delErr != nil {
		t.Fatalf("deleteRemote retry: %v", delErr)
	}
	if len(deleted) != 0 {
		t.Logf("deleted = %v (retry delivers the stranded tip without re-deleting)", deleted)
	}

	if got := remoteBranchSHA(t, url, branchSpec.PushBranch); got != stranded.String() {
		t.Fatalf("DEFECT: stranded deletion commit not delivered: remote feature tip = %q, stranded = %q", got, stranded)
	}
	if remoteFileExists(t, url, branchSpec.PushBranch, "inventory/team-a/inv.json") {
		t.Fatal("delivered stranded commit must retract the file")
	}
	if got := remoteBranchSHA(t, url, "main"); got != targetTip || got == "" {
		t.Fatalf("target branch must be untouched by the feature deletion: tip=%q, seed=%q", got, targetTip)
	}
}

// reviewC2 guard lock: a feature branch that never held work (created locally
// at the target tip, never pushed, no files anywhere) must stay a no-op — the
// stranded-tip delivery must never push a pointer-only feature branch that
// merge-request logic could mistake for a real deletion.
func TestDeleteRemote_MRMode_PointerOnlyBranchIsNoOp(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()
	branchSpec := &BranchSpec{PushBranch: "kollect/team-a/never", CloneBranch: "main"}

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-a/never-exported.json"}, branchSpec)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths,
		CommitContextFromObjectPath("inventory/team-a/never-exported.json", "prod"))
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want empty", deleted)
	}

	if sha := remoteBranchSHA(t, url, branchSpec.PushBranch); sha != "" {
		t.Fatalf("pointer-only feature branch must not be pushed, remote tip = %q", sha)
	}

	out, err := exec.Command("git", "ls-remote", url).CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("ls-remote: %s: %v", out, err)
	}
	if strings.Count(string(out), "refs/heads/main\n") != 1 {
		t.Fatalf("no-op delete must not add commits or branches, ls-remote:\n%s", out)
	}
}

// reviewF1 regression lock: inventory A has exported into the shared branchMR
// mirror, leaving its feature branch checked out as the mirror HEAD. Inventory
// B was never exported; deleting it matches nothing on disk, and the checkout
// synthesizes B's feature branch at A's tip. That tip must never be pushed as
// B's deletion: no B branch and no MR-worthy branch may appear, and A and the
// target branch stay untouched.
func TestDeleteRemote_MRMode_ForeignTipIsNoOp(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()

	exportSpec := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, exportSpec, CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	exportTip := remoteBranchSHA(t, url, exportSpec.PushBranch)
	targetTip := remoteBranchSHA(t, url, "main")
	if exportTip == "" || targetTip == "" {
		t.Fatalf("seed export did not land: feature=%q target=%q", exportTip, targetTip)
	}

	deleteSpec := &BranchSpec{PushBranch: "kollect/team-b/never", CloneBranch: "main"}
	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-b/never-exported.json"}, deleteSpec)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths,
		CommitContextFromObjectPath("inventory/team-b/never-exported.json", "prod"))
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want empty", deleted)
	}

	if sha := remoteBranchSHA(t, url, deleteSpec.PushBranch); sha != "" {
		t.Fatalf("DEFECT: foreign tip pushed as the deleted inventory's branch, remote tip = %q", sha)
	}
	if got := remoteBranchSHA(t, url, exportSpec.PushBranch); got != exportTip {
		t.Fatalf("other inventory's branch changed: tip=%q, want %q", got, exportTip)
	}
	if got := remoteBranchSHA(t, url, "main"); got != targetTip {
		t.Fatalf("target branch changed: tip=%q, want %q", got, targetTip)
	}
}

// reviewR1 regression lock: a pre-existing local feature branch that points at
// ANOTHER inventory's tip (a foreign export killed after its checkout, or a
// prior no-op delete that persisted the synthesized ref) must not be delivered
// as this inventory's deletion. The remote push branch is absent, so delivery
// is a no-op.
func TestDeleteRemote_MRMode_PoisonedForeignBranchIsNoOp(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()

	exportSpec := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, exportSpec, CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")); err != nil {
		t.Fatalf("seed export: %v", err)
	}
	exportTip := remoteBranchSHA(t, url, exportSpec.PushBranch)
	targetTip := remoteBranchSHA(t, url, "main")

	cloneURL, _, err := parseRemote(url)
	if err != nil {
		t.Fatalf("parseRemote: %v", err)
	}
	mirror, err := mirrorDirFor(cloneURL, "main")
	if err != nil {
		t.Fatalf("mirrorDirFor: %v", err)
	}

	deleteSpec := &BranchSpec{PushBranch: "kollect/team-b/inv", CloneBranch: "main"}
	if out, branchErr := exec.Command("git", "-C", mirror, "branch", deleteSpec.PushBranch).CombinedOutput(); branchErr != nil { //nolint:gosec // G204: test fixture touches its own mirror
		t.Fatalf("poison feature branch: %s: %v", out, branchErr)
	}

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-b/inv.json"}, deleteSpec)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths,
		CommitContextFromObjectPath("inventory/team-b/inv.json", "prod"))
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want empty", deleted)
	}

	if sha := remoteBranchSHA(t, url, deleteSpec.PushBranch); sha != "" {
		t.Fatalf("DEFECT: poisoned foreign branch pushed as this inventory's deletion, remote tip = %q", sha)
	}
	if got := remoteBranchSHA(t, url, exportSpec.PushBranch); got != exportTip {
		t.Fatalf("other inventory's branch changed: tip=%q, want %q", got, exportTip)
	}
	if got := remoteBranchSHA(t, url, "main"); got != targetTip {
		t.Fatalf("target branch changed: tip=%q, want %q", got, targetTip)
	}
}

// go-git analogue of the C1 staged-sweep defect: a crashed export's staged
// foreign file in the persistent mirror must never be committed by the go-git
// deletion path (the hard reset before the commit makes this deterministic —
// before the fix it relied on checkout's incidental index reset).
func TestDeleteRemote_GoGitSharedMirror_DoesNotSweepCrashedExportStagedAdds(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, nil, commitCtx); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	// Crashed foreign export: file written AND staged in the persistent mirror.
	cloneURL, _, err := parseRemote(url)
	if err != nil {
		t.Fatalf("parseRemote: %v", err)
	}
	mirror, err := mirrorDirFor(cloneURL, "main")
	if err != nil {
		t.Fatalf("mirrorDirFor: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(mirror, "inventory", "team-foreign"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mirror, "inventory", "team-foreign", "other.json"), []byte(`{"partial":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(mirror)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("mirror worktree: %v", err)
	}
	if _, err := wt.Add("inventory/team-foreign/other.json"); err != nil {
		t.Fatalf("stage crashed export: %v", err)
	}

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, nil)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths, commitCtx)
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the document", deleted)
	}

	if remoteFileExists(t, url, "main", "inventory/team-foreign/other.json") {
		t.Fatal("DEFECT: crashed export's staged file was committed to the user's repo under the deletion commit")
	}
	if remoteFileExists(t, url, "main", "inventory/team-a/inv.json") {
		t.Fatal("inventory/team-a/inv.json should have been removed from the remote")
	}
}

// reviewF1 removed>0 lock: when cleanup candidates DO match on disk the
// deletion commit must still base on the just-fetched clone tip, never on the
// foreign feature tip the mirror HEAD happens to point at. Otherwise the
// deleted inventory's branch carries another inventory's unmerged export.
func TestDeleteRemote_MRMode_DeletionCommitBasesOnCloneTipNotForeignTip(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url}.withDefaults()

	// Inventory B exported directly to the target branch (its MR was merged):
	// B's file now lives on main.
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-b/inv.json", Data: []byte(`{"items":[{"b":1}]}`)},
	}, nil, CommitContextFromObjectPath("inventory/team-b/inv.json", "prod")); err != nil {
		t.Fatalf("seed B export: %v", err)
	}
	targetTip := remoteBranchSHA(t, url, "main")

	// Inventory A exports to its unmerged feature branch, leaving the shared
	// mirror HEAD on A's tip.
	exportSpec := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, exportSpec, CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")); err != nil {
		t.Fatalf("seed A export: %v", err)
	}
	exportTip := remoteBranchSHA(t, url, exportSpec.PushBranch)

	// Delete B: its file matches on disk (present via main), so the deletion
	// commit is authored on top of whatever the checkout left HEAD at.
	deleteSpec := &BranchSpec{PushBranch: "kollect/team-b/inv", CloneBranch: "main"}
	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-b/inv.json"}, deleteSpec)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths,
		CommitContextFromObjectPath("inventory/team-b/inv.json", "prod"))
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want B's file", deleted)
	}

	if remoteBranchSHA(t, url, deleteSpec.PushBranch) == "" {
		t.Fatal("deletion branch was not pushed")
	}
	if remoteFileExists(t, url, deleteSpec.PushBranch, "inventory/team-a/inv.json") {
		t.Fatal("DEFECT: deletion branch carries another inventory's unmerged export")
	}
	if remoteFileExists(t, url, deleteSpec.PushBranch, "inventory/team-b/inv.json") {
		t.Fatal("deletion branch must retract B's file")
	}
	if parent := remoteParentSHA(t, url, deleteSpec.PushBranch); parent != targetTip {
		t.Fatalf("DEFECT: deletion commit based on %q, want clone tip %q", parent, targetTip)
	}
	if got := remoteBranchSHA(t, url, exportSpec.PushBranch); got != exportTip {
		t.Fatalf("A's branch changed: tip=%q, want %q", got, exportTip)
	}
	if got := remoteBranchSHA(t, url, "main"); got != targetTip {
		t.Fatalf("target branch changed: tip=%q, want %q", got, targetTip)
	}
}
