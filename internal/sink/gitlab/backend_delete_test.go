// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/sink/git"
)

// seedGitLabTestRemote creates a bare remote with one commit on main and
// returns (remotePath, runGitInRemote) for fixture tweaks.
func seedGitLabTestRemote(t *testing.T) (string, func(...string)) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")

	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"--git-dir", remote}, args...)...).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	initWork := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput() //nolint:gosec // G204: test fixture
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	initWork("init", "-b", "main", work)
	inWork := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", work}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	inWork("config", "user.name", "Kollect Tests")
	inWork("config", "user.email", "kollect-tests@example.com")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("seed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inWork("add", ".")
	inWork("commit", "-m", "seed")
	initWork("init", "--bare", remote)
	inWork("remote", "add", "origin", remote)
	inWork("push", "-u", "origin", "main")

	return remote, run
}

// K-29 shape at the API layer: a branchMR deletion that found nothing to remove
// must not open a merge request — its source branch was never created, so the
// MR call can only fail and wedge the cleanup retry loop with the inventory
// Terminating. The endpoint derives the API base, so a file:// endpoint makes
// any EnsureMergeRequest call fail loudly: a nil error proves it was skipped.
func TestBackend_DeleteExport_NoOpSkipsMergeRequest(t *testing.T) {
	remote, _ := seedGitLabTestRemote(t)

	b := &Backend{
		cfg: Config{
			Endpoint: "file://" + remote,
			MergeRequest: MergeRequestConfig{
				Mode:         MergeRequestModeBranchMR,
				TargetBranch: "main",
			},
		},
		auth: git.Auth{},
	}

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/never-exported.json"})
	if err != nil {
		t.Fatalf("no-op DeleteExport errored (would wedge deletion retries): %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want empty", deleted)
	}

	// No deletion commit may exist anywhere. (The CLI engine's stranded-commit
	// delivery may leave a pointer-only feature branch at the seed commit; the
	// go-git engine — gitlab's production path — leaves nothing.)
	out, err := exec.Command( //nolint:gosec // G204: test fixture inspects its own temp repo
		"git", "--git-dir", remote, "rev-list", "--count", "--all",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-list: %s: %v", out, err)
	}
	if got := strings.TrimSpace(string(out)); got != "1" {
		t.Fatalf("no-op delete added commits to the remote: rev-list --count --all = %s", got)
	}
}

// An earlier attempt may have pushed the deletion commit to the feature branch
// and died before the merge-request call. The retry finds nothing left to
// delete but must still open the MR: the probe sees the branch, so the MR step
// is reached (a file:// endpoint cannot serve the API, so reaching it fails
// loudly — a nil error here would mean the MR was skipped and the deletion
// commit orphaned with the inventory gone).
func TestBackend_DeleteExport_BranchMR_ReopensMergeRequestForStrandedBranch(t *testing.T) {
	remote, runRemote := seedGitLabTestRemote(t)

	runRemote("update-ref", "refs/heads/kollect/team-a/never-exported", "HEAD")

	b := &Backend{
		cfg: Config{
			Endpoint: "file://" + remote,
			MergeRequest: MergeRequestConfig{
				Mode:         MergeRequestModeBranchMR,
				TargetBranch: "main",
			},
		},
		auth: git.Auth{},
	}

	_, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/never-exported.json"})
	if err == nil {
		t.Fatal("no-op delete with a stranded feature branch must still attempt the merge request")
	}
}
