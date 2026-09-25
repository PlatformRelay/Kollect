// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func skipWithoutGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

func cloneVerifyDir(t *testing.T, remote string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "verify")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", remote, dir).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture clone
		t.Fatalf("clone remote: %s: %v", out, err)
	}

	return dir
}

// K-28 test lock (tombstone): inventory deletion removes the exported document
// and its part siblings from the git remote in a deletion commit, and never
// touches a sibling inventory whose name shares the prefix.
func TestDeleteExportWithBranch_RemovesDocumentAndPartSiblings(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")
	files := []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-a/inv.part-0001-of-0002.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-a/inv.part-0002-of-0002.json", Data: []byte(`{"items":[{"a":2}]}`)},
		{Path: "inventory/team-a/inv-v2.json", Data: []byte(`{"items":[{"other":true}]}`)},
		{Path: "inventory/team-b/inv.json", Data: []byte(`{"items":[{"ns2":true}]}`)},
	}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, commitCtx); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	deleted, delErr := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/inv.json"}, nil, commitCtx)
	if delErr != nil {
		t.Fatalf("DeleteExportWithBranch: %v", delErr)
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted = %v, want the document and its two part siblings", deleted)
	}

	verify := cloneVerifyDir(t, remote)

	for _, gone := range []string{
		"inventory/team-a/inv.json",
		"inventory/team-a/inv.part-0001-of-0002.json",
		"inventory/team-a/inv.part-0002-of-0002.json",
	} {
		if _, err := os.Stat(filepath.Join(verify, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed from the remote", gone)
		}
	}

	for _, kept := range []string{
		"inventory/team-a/inv-v2.json",
		"inventory/team-b/inv.json",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(verify, filepath.FromSlash(kept))); err != nil {
			t.Errorf("%s must survive cleanup: %v", kept, err)
		}
	}

	logOut, err := exec.Command("git", "-C", verify, "log", "--format=%s").CombinedOutput() //nolint:gosec // G204: test fixture inspects its own temp repo
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(logOut), "remove inventory export") {
		t.Fatalf("expected a deletion commit, log subjects:\n%s", logOut)
	}
}

// Deletion must be idempotent: no matching files means no commit and no error.
func TestDeleteExportWithBranch_NothingMatchedIsNoop(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	deleted, delErr := DeleteExportWithBranch(t.Context(), cfg, Auth{}, []string{"inventory/team-a/never-exported.json"}, nil,
		CommitContextFromObjectPath("inventory/team-a/never-exported.json", "prod"))
	if delErr != nil || len(deleted) != 0 {
		t.Fatalf("DeleteExportWithBranch on empty remote = %v/%v, want empty/nil", deleted, delErr)
	}

	// The seed commit is the only commit; a no-op delete must not add one.
	out, err := exec.Command("git", "--git-dir", remote, "rev-list", "--count", "main").CombinedOutput() //nolint:gosec // G204: test fixture inspects its own temp repo
	if err != nil {
		t.Fatalf("rev-list: %s: %v", out, err)
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("no-op delete created commits on the remote: rev-list --count = %s", strings.TrimSpace(string(out)))
	}
}

// go-git engine (direct deleteRemote) must retract the same file set.
func TestDeleteRemote_GoGitEngineRemovesFiles(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	commitCtx := CommitContextFromObjectPath("inventory/team-a/inv.json", "prod")

	seedReq, seedFiles, err := validateExportFiles(cfg, []FileEntry{
		{Path: "inventory/team-a/inv.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-a/inv.part-0001-of-0001.json", Data: []byte(`{"items":[{"a":1}]}`)},
		{Path: "inventory/team-b/inv.json", Data: []byte(`{"items":[{"b":1}]}`)},
	}, nil)
	if err != nil {
		t.Fatalf("validate seed: %v", err)
	}

	if seedErr := exportRemote(t.Context(), cfg, Auth{}, seedReq, seedFiles, commitCtx); seedErr != nil {
		t.Fatalf("seed exportRemote: %v", seedErr)
	}

	req, paths, delErr := validateDeletePaths(cfg, []string{"inventory/team-a/inv.json"}, nil)
	if delErr != nil {
		t.Fatalf("validateDeletePaths: %v", delErr)
	}

	deleteCfg := cfg
	deleteCfg.CommitMessage = deleteCommitMessage

	deleted, delErr := deleteRemote(t.Context(), deleteCfg, Auth{}, req, paths, commitCtx)
	if delErr != nil {
		t.Fatalf("deleteRemote: %v", delErr)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted = %v, want the document and its part sibling", deleted)
	}

	verify := cloneVerifyDir(t, remote)

	if _, statErr := os.Stat(filepath.Join(verify, "inventory/team-a/inv.json")); !os.IsNotExist(statErr) {
		t.Errorf("inventory/team-a/inv.json should have been removed")
	}
	if _, statErr := os.Stat(filepath.Join(verify, "inventory/team-a/inv.part-0001-of-0001.json")); !os.IsNotExist(statErr) {
		t.Errorf("part sibling should have been removed")
	}
	if _, statErr := os.Stat(filepath.Join(verify, "inventory/team-b/inv.json")); statErr != nil {
		t.Errorf("sibling inventory file must survive: %v", statErr)
	}

	logOut, err := exec.Command("git", "-C", verify, "log", "--format=%s").CombinedOutput() //nolint:gosec // G204: test fixture inspects its own temp repo
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(logOut), "remove inventory export") {
		t.Fatalf("expected a deletion commit on the go-git engine, log:\n%s", logOut)
	}
}
