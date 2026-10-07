// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os/exec"
	"strings"
	"testing"
)

// gitBranchHasPath reports whether refs/heads/<branch> of a bare remote holds path.
func gitBranchHasPath(t *testing.T, remote, branch, path string) bool {
	t.Helper()

	return exec.Command("git", "--git-dir", remote, "cat-file", "-e", "refs/heads/"+branch+":"+path).Run() == nil //nolint:gosec // G204: test fixture
}

func gitBranchTip(t *testing.T, remote, branch string) string {
	t.Helper()

	out, err := exec.Command("git", "--git-dir", remote, "rev-parse", "refs/heads/"+branch).CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("rev-parse %s: %s: %v", branch, out, err)
	}

	return strings.TrimSpace(string(out))
}

// seedUnmergedFeatureExport exports the inventory to its merge-request feature
// branch only: the target branch never receives it (the MR is unmerged).
func seedUnmergedFeatureExport(t *testing.T, cfg Config, branch *BranchSpec) {
	t.Helper()

	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
	}, branch, CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")); err != nil {
		t.Fatalf("seed feature export: %v", err)
	}
}

// MR-06 regression lock, CLI engine: with a cold mirror (file:// remotes get a
// fresh workdir per operation, as does a restarted pod) the local feature branch
// does not exist, so it used to be synthesized from the target branch, where
// the unmerged export is absent: the deletion found nothing, and the merge
// request kept proposing the deleted inventory's snapshot. The deletion must
// base on the remote feature branch and retract the file there.
func TestDeleteExportWithBranch_CLIColdMirror_RetractsFromRemoteFeatureBranch(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()
	branch := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}

	seedUnmergedFeatureExport(t, cfg, branch)
	targetTip := gitBranchTip(t, remote, "main")

	deleted, err := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/inv.json"}, branch,
		CommitContextFromObjectPath("inventory/team-a/inv.json", "prod"))
	if err != nil {
		t.Fatalf("DeleteExportWithBranch: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the feature branch's export", deleted)
	}
	if gitBranchHasPath(t, remote, branch.PushBranch, "inventory/team-a/inv.json") {
		t.Fatal("DEFECT: the feature branch (and its MR) still adds the deleted inventory's snapshot")
	}
	if tip := gitBranchTip(t, remote, "main"); tip != targetTip {
		t.Fatalf("target branch changed: %s, want %s", tip, targetTip)
	}
}

// MR-06 regression lock, go-git engine.
func TestDeleteRemote_ColdMirror_RetractsFromRemoteFeatureBranch(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()
	branch := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}

	seedUnmergedFeatureExport(t, cfg, branch)
	targetTip := gitBranchTip(t, remote, "main")

	req, paths, err := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, branch)
	if err != nil {
		t.Fatalf("validateDeletePaths: %v", err)
	}
	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, err := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths, CommitContextFromObjectPath("inventory/team-a/inv.json", "prod"))
	if err != nil {
		t.Fatalf("deleteRemote: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the feature branch's export", deleted)
	}
	if gitBranchHasPath(t, remote, branch.PushBranch, "inventory/team-a/inv.json") {
		t.Fatal("DEFECT: the feature branch (and its MR) still adds the deleted inventory's snapshot")
	}
	if tip := gitBranchTip(t, remote, "main"); tip != targetTip {
		t.Fatalf("target branch changed: %s, want %s", tip, targetTip)
	}
}
