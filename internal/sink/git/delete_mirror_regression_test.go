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

// reviewC1/F1 regression lock: with the CLI engine against a persistent shared
// warm mirror, a crashed export leaves STAGED foreign files in the mirror index.
// The deletion of a DIFFERENT inventory must not commit that foreign state into
// the user's repo under the deletion commit's subject, and must still deliver
// its own deletion.
func TestDeleteExportWithBranch_CLISharedMirror_DoesNotSweepCrashedExportStagedAdds(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url, Engine: GitEngineCLI}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")
	files := []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-a/inv.part-0001-of-0002.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-a/inv.part-0002-of-0002.json", Data: []byte(`{"items":[{"a":2}]}`)},
	}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, commitCtx); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	// Simulate the crashed foreign export at the mirror: files written and
	// staged (git add), commit never reached. Same mirror as team-a's (keyed
	// by clone URL + clone branch).
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
	if out, err := exec.Command("git", "-C", mirror, "add", "inventory/team-foreign/other.json").CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture touches its own mirror
		t.Fatalf("stage crashed export: %s: %v", out, err)
	}

	deleted, delErr := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/inv.json"}, nil, commitCtx)
	if delErr != nil {
		t.Fatalf("DeleteExportWithBranch: %v", delErr)
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted = %v, want the document and its two part siblings", deleted)
	}

	if remoteFileExists(t, url, "main", "inventory/team-foreign/other.json") {
		t.Fatal("DEFECT: crashed export's staged file was committed to the user's repo under the deletion commit")
	}
	if remoteFileExists(t, url, "main", "inventory/team-a/inv.json") {
		t.Fatal("inventory/team-a/inv.json should have been removed from the remote")
	}
	if !remoteFileExists(t, url, "main", "README.md") {
		t.Fatal("README.md must survive cleanup")
	}
	if subjects := remoteLogSubjects(t, url, "main"); !strings.Contains(subjects, "remove inventory export") {
		t.Fatalf("expected the deletion commit, log subjects:\n%s", subjects)
	}
}

// reviewF2 regression lock (merge blocker): with the CLI engine against a
// persistent shared warm mirror, an UNTRACKED foreign leftover (crashed export
// between write and stage) plus a deletion that matches nothing must be an
// honest no-op — not a nothing-to-commit error looping forever, invisible to
// the terminal-cleanup alert.
func TestDeleteExportWithBranch_CLISharedMirror_UntrackedDirtIsNoOp(t *testing.T) {
	root := t.TempDir()
	srv := startGitHTTPServer(t, root)
	url := seedBareRepo(t, srv, "remote.git")
	mirrorIsolate(t)

	cfg := Config{Endpoint: url, Engine: GitEngineCLI}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, nil, commitCtx); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	cloneURL, _, err := parseRemote(url)
	if err != nil {
		t.Fatalf("parseRemote: %v", err)
	}
	mirror, err := mirrorDirFor(cloneURL, "main")
	if err != nil {
		t.Fatalf("mirrorDirFor: %v", err)
	}
	// Untracked foreign leftover: written, never staged, never committed.
	if err := os.MkdirAll(filepath.Join(mirror, "inventory", "team-foreign"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mirror, "inventory", "team-foreign", "orphan.json"), []byte(`{"partial":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	deleted, delErr := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-zz/never-exported.json"}, nil,
		CommitContextFromObjectPath("inventory/team-zz/never-exported.json", "prod"))
	if delErr != nil {
		t.Fatalf("DeleteExportWithBranch wedged on foreign untracked dirt: %v", delErr)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want empty", deleted)
	}

	if remoteFileExists(t, url, "main", "inventory/team-foreign/orphan.json") {
		t.Fatal("foreign untracked leftover must never be committed to the user's repo")
	}

	out, err := exec.Command("git", "ls-remote", url).CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("ls-remote: %s: %v", out, err)
	}
	if strings.Count(string(out), "refs/heads/main\n") != 1 {
		t.Fatalf("no-op delete must not add commits or branches, ls-remote:\n%s", out)
	}
}

// reviewC2 regression lock: go-git engine (gitlab's production engine) in
// branchMR mode. A crash between the deletion commit and its push leaves the
// deletion commit stranded on the mirror's feature-branch ref, which
// openOrWarmMirror never reconciles (it force-fetches the CLONE branch only).
// The retry must DELIVER the stranded tip, not silently orphan it while
// reporting a no-op.
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
