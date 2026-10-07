// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"path/filepath"
	"strings"
	"testing"
)

// Engine-convergence regressions (T04, GTE-1 scenario 5): file:// remotes and the connection
// probe keep their current behaviour through the engine convergence. Every fixture here is
// engine-less on purpose — no Config{Engine: ...} and no GitEngineCLI constant, both of which
// T09 removes — so these tests pass before and after the API change unchanged.

// Engine-less file:// export lands the file on the remote branch, as today.
func TestFileRemote_DefaultEngine_ExportLandsFile(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	files := []FileEntry{{Path: "inventory/latest.json", Data: []byte(`{"x":1}`)}}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{}); err != nil {
		t.Fatalf("ExportFilesWithBranch(file:// remote) error = %v, want the export to land as today", err)
	}

	if !gitBranchHasPath(t, remote, "main", "inventory/latest.json") {
		t.Fatal("the exported file did not land on the remote's main branch (want unchanged file:// export behaviour)")
	}
}

// Engine-less file:// delete retracts from the remote feature branch and leaves the target
// branch untouched (MR-06 mechanics without the Engine field), as today.
func TestFileRemote_DefaultEngine_DeleteRetractsFromFeatureBranch(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()
	branch := &BranchSpec{PushBranch: "kollect/team-a/inv", CloneBranch: "main"}

	seedUnmergedFeatureExport(t, cfg, branch)
	targetTip := gitBranchTip(t, remote, "main")

	deleted, err := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/inv.json"}, branch,
		CommitContextFromObjectPath("inventory/team-a/inv.json", "prod"))
	if err != nil {
		t.Fatalf("DeleteExportWithBranch(file:// remote) error = %v, want the deletion to land as today", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the feature branch's export", deleted)
	}
	if gitBranchHasPath(t, remote, branch.PushBranch, "inventory/team-a/inv.json") {
		t.Fatal("the feature branch still holds the deleted inventory's snapshot (want the retraction as today)")
	}
	if tip := gitBranchTip(t, remote, "main"); tip != targetTip {
		t.Fatalf("target branch changed: %s, want %s (the deletion must not touch the clone branch)", tip, targetTip)
	}
}

// Engine-less file:// connection probe: a valid bare remote probes clean, and a non-repo path
// fails with the probe's own "git ls-remote failed:" wrapper — the CLI probe taxonomy, not a
// go-git error — as today.
func TestConnectionProbe_FileRemote_DefaultEngine(t *testing.T) {
	skipWithoutGit(t)

	t.Run("valid bare remote probes clean", func(t *testing.T) {
		remote := createBareRemoteWithMainCommit(t)
		cfg := Config{Endpoint: "file://" + remote}.withDefaults()
		if err := TestConnection(t.Context(), cfg, Auth{}); err != nil {
			t.Fatalf("TestConnection(file:// remote) error = %v, want nil as today", err)
		}
	})

	t.Run("non-repo path fails through the ls-remote wrapper", func(t *testing.T) {
		notARepo := filepath.Join(t.TempDir(), "not-a-repo.git")
		cfg := Config{Endpoint: "file://" + notARepo}.withDefaults()

		err := TestConnection(t.Context(), cfg, Auth{})
		if err == nil {
			t.Fatal("TestConnection(file:// non-repo) error = nil, want the ls-remote failure as today")
		}
		if msg := err.Error(); !strings.Contains(msg, "git ls-remote failed:") {
			t.Fatalf("TestConnection(file:// non-repo) error = %q, want the CLI probe's own \"git ls-remote failed:\" wrapper", msg)
		}
	})
}
