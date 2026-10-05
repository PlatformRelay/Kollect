// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// reviewG1 lock: a crashed go-git export can leave an untracked candidate file
// in the shared mirror (written before staging). removeWorktreeCandidates must
// tolerate it: the disk removal is the whole effect, no error is raised, and it
// stages no index change, so the caller never attempts an empty deletion commit.
func TestRemoveWorktreeCandidates_UntrackedDirtIsTolerated(t *testing.T) {
	dir := t.TempDir()
	repo, initErr := git.PlainInit(dir, false)
	if initErr != nil {
		t.Fatalf("PlainInit: %v", initErr)
	}
	wt, wtErr := repo.Worktree()
	if wtErr != nil {
		t.Fatalf("Worktree: %v", wtErr)
	}

	invDir := filepath.Join(dir, "inventory")
	if err := os.MkdirAll(invDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Tracked candidate that the deletion must remove and commit.
	tracked := filepath.Join(invDir, "team-a", "inv.json")
	if err := os.MkdirAll(filepath.Dir(tracked), 0o750); err != nil {
		t.Fatalf("MkdirAll(team-a): %v", err)
	}
	if err := os.WriteFile(tracked, []byte(`{"items":[{"a":1}]}`), 0o600); err != nil {
		t.Fatalf("WriteFile(tracked): %v", err)
	}
	if _, err := wt.Add("inventory/team-a/inv.json"); err != nil {
		t.Fatalf("Add(tracked): %v", err)
	}
	if _, err := wt.Commit("seed", &git.CommitOptions{
		Author: &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com"},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Untracked crashed-export leftover for another inventory: written but never
	// staged, so it is absent from the index.
	untracked := filepath.Join(invDir, "team-b", "inv.json")
	if err := os.MkdirAll(filepath.Dir(untracked), 0o750); err != nil {
		t.Fatalf("MkdirAll(team-b): %v", err)
	}
	if err := os.WriteFile(untracked, []byte(`{"items":[{"b":1}]}`), 0o600); err != nil {
		t.Fatalf("WriteFile(untracked): %v", err)
	}

	removed, rmErr := removeWorktreeCandidates(wt, []string{
		"inventory/team-b/inv.json",
		"inventory/team-a/inv.json",
	}, nil)
	if rmErr != nil {
		t.Fatalf("DEFECT: untracked dirt errored the go-git deletion: %v", rmErr)
	}

	if len(removed) != 1 || removed[0] != "inventory/team-a/inv.json" {
		t.Fatalf("removed = %v, want only the tracked candidate", removed)
	}
	for _, p := range []string{tracked, untracked} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Fatalf("candidate %q still on disk (stat err = %v)", p, statErr)
		}
	}

	status, stErr := wt.Status()
	if stErr != nil {
		t.Fatalf("Status: %v", stErr)
	}
	if st, ok := status["inventory/team-a/inv.json"]; !ok || st.Staging != git.Deleted {
		t.Fatalf("tracked candidate must be staged as deleted, status = %v", status)
	}
	if _, ok := status["inventory/team-b/inv.json"]; ok {
		t.Fatalf("untracked candidate must leave no status entry, status = %v", status)
	}
}
