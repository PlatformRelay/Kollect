// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestStageChanges_NoPruneAddsSpecifiedPaths(t *testing.T) {
	t.Parallel()

	_, wt, dir := initRepoWithSeedCommit(t)
	target := filepath.Join(dir, "inventory", "next.json")
	if err := os.WriteFile(target, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(target): %v", err)
	}

	if err := stageChanges(wt, []string{"inventory/next.json"}, false); err != nil {
		t.Fatalf("stageChanges() error = %v", err)
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if st := status.File("inventory/next.json"); st.Staging == git.Untracked {
		t.Fatalf("inventory/next.json staging = %v, want added", st.Staging)
	}
}

func TestStageChanges_PruneStagesDeletesAndUpdates(t *testing.T) {
	t.Parallel()

	_, wt, dir := initRepoWithSeedCommit(t)
	if err := os.WriteFile(filepath.Join(dir, "inventory", "keep.json"), []byte(`{"updated":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(keep): %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "inventory", "old.json")); err != nil {
		t.Fatalf("Remove(old): %v", err)
	}

	if err := stageChanges(wt, []string{"inventory/keep.json"}, true); err != nil {
		t.Fatalf("stageChanges() error = %v", err)
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if st := status.File("inventory/keep.json"); st.Staging == git.Untracked {
		t.Fatalf("inventory/keep.json staging = %v, want tracked change", st.Staging)
	}
	if st := status.File("inventory/old.json"); st.Staging != git.Deleted && st.Worktree != git.Deleted {
		t.Fatalf("inventory/old.json status = %+v, want deleted", st)
	}
}

func TestPushCommitted_WrapsMissingOriginError(t *testing.T) {
	t.Parallel()

	repo, wt, dir := initRepoWithSeedCommit(t)
	if err := os.WriteFile(filepath.Join(dir, "inventory", "keep.json"), []byte(`{"seed":false}`), 0o600); err != nil {
		t.Fatalf("WriteFile(keep update): %v", err)
	}
	if _, err := wt.Add("inventory/keep.json"); err != nil {
		t.Fatalf("Add(keep update): %v", err)
	}
	hash, err := wt.Commit("second", &git.CommitOptions{
		Author: &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com"},
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	err = pushCommitted(t.Context(), repo, Config{}, nil, "file:///tmp/remote.git", "main", false, hash, wt)
	if err == nil || !strings.Contains(err.Error(), "remote origin") {
		t.Fatalf("pushCommitted() error = %v, want missing origin error", err)
	}
}

func TestIsFileRemote(t *testing.T) {
	t.Parallel()

	if !isFileRemote("file:///tmp/repo.git") {
		t.Fatal("expected file:// URL to be recognized as a file remote")
	}
	if isFileRemote("https://example.com/repo.git") {
		t.Fatal("expected https:// URL to not be a file remote")
	}
	if isFileRemote("\x7f://bad") {
		t.Fatal("expected unparseable URL to not be a file remote")
	}
}

func TestIsEmptyRemote_RecognizesKnownMessages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: false},
		{err: errors.New("remote repository is empty"), want: true},
		{err: errors.New("couldn't find remote ref main"), want: true},
		{err: errors.New("reference not found"), want: true},
		{err: errors.New("permission denied"), want: false},
	}
	for _, tc := range cases {
		if got := isEmptyRemote(tc.err); got != tc.want {
			t.Fatalf("isEmptyRemote(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func initRepoWithSeedCommit(t *testing.T) (*git.Repository, *git.Worktree, string) {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "inventory"), 0o750); err != nil {
		t.Fatalf("MkdirAll(inventory): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory", "keep.json"), []byte(`{"seed":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(keep): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory", "old.json"), []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(old): %v", err)
	}
	if _, err := wt.Add("inventory/keep.json"); err != nil {
		t.Fatalf("Add(keep): %v", err)
	}
	if _, err := wt.Add("inventory/old.json"); err != nil {
		t.Fatalf("Add(old): %v", err)
	}
	if _, err := wt.Commit("seed", &git.CommitOptions{
		Author: &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com"},
	}); err != nil {
		t.Fatalf("Commit(seed): %v", err)
	}
	return repo, wt, dir
}
