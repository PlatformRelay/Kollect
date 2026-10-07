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
)

func skipWithoutGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

// gitOutput runs a git command in cwd (when non-empty) and returns stdout,
// failing the test on a non-zero exit. It is the output-capturing companion to
// runGit/runGitC for fixtures that must inspect refs and hashes.
func gitOutput(t *testing.T, cwd string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...) //nolint:gosec // test helper executes static git fixture commands
	if cwd != "" {
		cmd.Dir = cwd
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}

	return strings.TrimSpace(string(out))
}

func initCLIFixtureMirror(t *testing.T, remote string) string {
	t.Helper()

	work := filepath.Join(t.TempDir(), "mirror")
	runGit(t, "clone", remote, work)
	runGitC(t, work, "config", "user.name", "Kollect Tests")
	runGitC(t, work, "config", "user.email", "kollect-tests@example.com")

	return work
}

// reviewR3/F2 lock: a shared warm mirror holding uncommitted tracked dirt (a
// file absent from the target branch) and untracked leftovers must not block
// the CLI branch switch. prepareCLIWorkdir clears the mirror before checkout so
// the deletion of another inventory completes and the dirt is discarded.
func TestPrepareCLIWorkdir_WarmMirrorDiscardsDirtBeforeCheckout(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	// The mirror HEAD sits on another inventory's branch holding a file that is
	// absent from the clone branch (main). A dirty modification of that file is
	// exactly what blocks `git checkout` (and `checkout -B ... origin/main`).
	runGitC(t, work, "checkout", "-b", "other")
	mustWriteFile(t, filepath.Join(work, "foreign.txt"), []byte("foreign\n"))
	runGitC(t, work, "add", "foreign.txt")
	runGitC(t, work, "commit", "-m", "foreign export")

	mustWriteFile(t, filepath.Join(work, "foreign.txt"), []byte("dirty\n"))
	mustWriteFile(t, filepath.Join(work, "leftover.txt"), []byte("junk\n"))

	if err := prepareCLIWorkdir(t.Context(), work, remote, "main", "feature", Config{}.withDefaults(), nil); err != nil {
		t.Fatalf("prepareCLIWorkdir blocked by mirror dirt: %v", err)
	}

	if branch := gitOutput(t, work, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature" {
		t.Fatalf("HEAD = %q, want the push branch feature", branch)
	}
	// The synthesized branch must be based on the clone tip, never the foreign
	// mirror HEAD: foreign.txt is not on main and must be gone.
	if _, err := os.Stat(filepath.Join(work, "foreign.txt")); !os.IsNotExist(err) {
		t.Errorf("foreign.txt survived: the push branch was not based on the clone tip (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(work, "leftover.txt")); !os.IsNotExist(err) {
		t.Errorf("untracked leftover.txt survived: git clean -fd did not run (stat err = %v)", err)
	}
	if status := gitOutput(t, work, "status", "--porcelain"); status != "" {
		t.Errorf("mirror still dirty after prepareCLIWorkdir:\n%s", status)
	}
}

// An existing push branch is checked out as-is by the CLI machinery: `checkout -B`
// would reset it to the mirror's HEAD (possibly another inventory's tip) and
// drop a commit it still has to deliver.
func TestGitCheckoutPushBranch_KeepsExistingBranchTip(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	runGitC(t, work, "checkout", "-q", "-b", "feature")
	mustWriteFile(t, filepath.Join(work, "stranded.txt"), []byte("stranded\n"))
	runGitC(t, work, "add", "stranded.txt")
	runGitC(t, work, "commit", "-q", "-m", "stranded")
	stranded := gitOutput(t, work, "rev-parse", "HEAD")

	// The mirror HEAD moves to another inventory's branch before the next run.
	runGitC(t, work, "checkout", "-q", "-b", "other", "main")
	mustWriteFile(t, filepath.Join(work, "other.txt"), []byte("other\n"))
	runGitC(t, work, "add", "other.txt")
	runGitC(t, work, "commit", "-q", "-m", "other")

	if err := gitCheckoutPushBranch(t.Context(), work, "main", "feature", nil); err != nil {
		t.Fatalf("gitCheckoutPushBranch: %v", err)
	}
	if head := gitOutput(t, work, "rev-parse", "HEAD"); head != stranded {
		t.Fatalf("existing push branch was reset: HEAD=%s, want its own tip %s", head, stranded)
	}
}

// go-git path: a missing push branch is created at the clone-branch tip, not
// at the mirror's current HEAD; an existing one keeps its tip.
func TestCheckoutMirrorBranch_BasesNewBranchOnCloneTip(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)
	mainTip := gitOutput(t, work, "rev-parse", "main")

	// Mirror HEAD on another inventory's branch with a commit main lacks.
	runGitC(t, work, "checkout", "-q", "-b", "other")
	mustWriteFile(t, filepath.Join(work, "other.txt"), []byte("other\n"))
	runGitC(t, work, "add", "other.txt")
	runGitC(t, work, "commit", "-q", "-m", "other")
	otherTip := gitOutput(t, work, "rev-parse", "HEAD")

	repo, err := git.PlainOpen(work)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if err = checkoutMirrorBranch(repo, wt, "main", "feature"); err != nil {
		t.Fatalf("checkoutMirrorBranch(new): %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Name() != plumbing.NewBranchReferenceName("feature") || head.Hash().String() != mainTip {
		t.Fatalf("new push branch = %s@%s, want feature@%s (clone tip, not %s)", head.Name(), head.Hash(), mainTip, otherTip)
	}

	if err = checkoutMirrorBranch(repo, wt, "main", "other"); err != nil {
		t.Fatalf("checkoutMirrorBranch(existing): %v", err)
	}
	if head, err = repo.Head(); err != nil || head.Hash().String() != otherTip {
		t.Fatalf("existing branch moved: %v (err %v), want %s", head, err, otherTip)
	}
}
