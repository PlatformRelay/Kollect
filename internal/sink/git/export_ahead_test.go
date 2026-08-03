// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bareRef reads the commit a bare repository has for refs/heads/<branch>, or ""
// when the branch is absent. It is C-locale pinned so the assertion is
// independent of the operator's ambient locale (REL-01).
func bareRef(t *testing.T, bare, branch string) string {
	t.Helper()

	cmd := exec.Command("git", "--git-dir", bare, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) //nolint:gosec // G204: test fixture
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "LANG=C")
	out, _ := cmd.Output()

	return strings.TrimSpace(string(out))
}

// TestSyncCLIWorkdir_pushesStrandedCommitWhenTreeClean reproduces REL-06: an
// earlier export committed a snapshot locally but its push never reached the
// remote. On the next run the inventory is unchanged, so the working tree is
// clean -- yet the local branch is ahead of origin/<pushBranch>. The sync must
// still push so the remote ends up holding the local HEAD, and it must not
// report success (return nil without delivering) while the commit is stranded.
func TestSyncCLIWorkdir_pushesStrandedCommitWhenTreeClean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	bare := createBareRemoteWithMainCommit(t)
	cloneURL := "file://" + bare
	c0 := bareRef(t, bare, "main")

	workdir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "clone", "--branch", "main", "--single-branch", cloneURL, workdir)
	runGitC(t, workdir, "config", "user.name", "Kollect Tests")
	runGitC(t, workdir, "config", "user.email", "kollect-tests@example.com")

	// Commit a new snapshot locally but deliberately do NOT push it: this is the
	// stranded state a failed/interrupted push leaves behind in a warm mirror.
	mustWriteFile(t, filepath.Join(workdir, "inventory/latest.json"), []byte(`{"items":[{"uid":"u1"}]}`))
	runGitC(t, workdir, "add", ".")
	runGitC(t, workdir, "commit", "-m", "stranded snapshot")

	local := strings.TrimSpace(gitRevParseHeadForTest(t, workdir))
	if local == "" || local == c0 {
		t.Fatalf("test setup: expected a new local commit ahead of remote, local=%q c0=%q", local, c0)
	}
	if got := bareRef(t, bare, "main"); got != c0 {
		t.Fatalf("test setup: remote main should still be at C0 %q before sync, got %q", c0, got)
	}

	cfg := Config{Endpoint: cloneURL, PushPolicy: PushPolicyCommit}
	if err := syncCLIWorkdir(t.Context(), workdir, cloneURL, "main", cfg, CommitContext{Checksum: "c1"}, nil); err != nil {
		t.Fatalf("syncCLIWorkdir() error = %v", err)
	}

	// Synced=true (nil return) is only legitimate once the remote actually holds
	// the snapshot: the bare's main must now equal the local HEAD.
	if got := bareRef(t, bare, "main"); got != local {
		t.Fatalf("remote main = %q, want local HEAD %q -- stranded commit was not delivered", got, local)
	}
}

// TestSyncCLIWorkdir_noPushWhenRemoteAlreadyHasHead is the other direction of the REL-06
// contract: when the remote already holds the local HEAD and the tree is clean, the sync reports
// success without pushing and without fabricating an empty commit. This pins the discriminating
// logic -- an "always push" implementation would still pass the stranded-commit test (push is a
// no-op on a synced repo), so this case is what proves the remote-has-head gate actually gates.
func TestSyncCLIWorkdir_noPushWhenRemoteAlreadyHasHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	bare := createBareRemoteWithMainCommit(t)
	cloneURL := "file://" + bare

	workdir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "clone", "--branch", "main", "--single-branch", cloneURL, workdir)
	runGitC(t, workdir, "config", "user.name", "Kollect Tests")
	runGitC(t, workdir, "config", "user.email", "kollect-tests@example.com")

	// Commit a snapshot AND push it: remote and local now agree, tree is clean.
	mustWriteFile(t, filepath.Join(workdir, "inventory/latest.json"), []byte(`{"items":[{"uid":"u1"}]}`))
	runGitC(t, workdir, "add", ".")
	runGitC(t, workdir, "commit", "-m", "delivered snapshot")
	runGitC(t, workdir, "push", "origin", "main")

	local := strings.TrimSpace(gitRevParseHeadForTest(t, workdir))
	if got := bareRef(t, bare, "main"); got != local {
		t.Fatalf("test setup: remote main should equal local HEAD %q after push, got %q", local, got)
	}

	cfg := Config{Endpoint: cloneURL, PushPolicy: PushPolicyCommit}
	if err := syncCLIWorkdir(t.Context(), workdir, cloneURL, "main", cfg, CommitContext{Checksum: "c1"}, nil); err != nil {
		t.Fatalf("syncCLIWorkdir() error = %v", err)
	}

	// Already-synced: no new commit fabricated locally, remote unchanged.
	if got := strings.TrimSpace(gitRevParseHeadForTest(t, workdir)); got != local {
		t.Fatalf("local HEAD changed to %q, want unchanged %q -- sync must not fabricate a commit", got, local)
	}
	if got := bareRef(t, bare, "main"); got != local {
		t.Fatalf("remote main = %q, want unchanged %q -- sync must not push when already delivered", got, local)
	}
}

// TestSyncCLIWorkdir_abortsOnLsRemoteFailure locks the REL-06 fail-safe (REL-06-FUP): when
// `git ls-remote` cannot reach the remote, the sync must ABORT with a non-nil error rather than
// swallow the failure and either report success or push a stranded snapshot it could not verify.
// Reporting Synced=true (returning nil) while unable to confirm the remote's state is exactly the
// false-synced bug REL-06 closed, so the fail-safe must hold on ls-remote failure.
//
// The harness splits the origin's URLs to isolate the failure to the read path only:
//   - remote.origin.url points at a nonexistent bare (exit 128) so `ls-remote origin` FAILS;
//   - remote.origin.pushurl points at the real bare so a hypothetical push would SUCCEED.
//
// This split is what makes the test RED-meaningful. A regression that swallows the ls-remote error
// (returning not-synced instead of propagating) would fall through and push to the valid pushurl,
// advancing the remote to the stranded local HEAD and returning nil. Both signals below then fire:
// the error goes nil, and the remote advances off C0. On the correct fail-safe the remote is never
// touched because the sync aborts before the push.
func TestSyncCLIWorkdir_abortsOnLsRemoteFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	bare := createBareRemoteWithMainCommit(t)
	cloneURL := "file://" + bare
	c0 := bareRef(t, bare, "main")

	workdir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "clone", "--branch", "main", "--single-branch", cloneURL, workdir)
	runGitC(t, workdir, "config", "user.name", "Kollect Tests")
	runGitC(t, workdir, "config", "user.email", "kollect-tests@example.com")

	// Commit a stranded snapshot locally (ahead of remote) but do NOT push it; the tree is then
	// clean, so the sync enters the remote-has-head check where ls-remote is consulted.
	mustWriteFile(t, filepath.Join(workdir, "inventory/latest.json"), []byte(`{"items":[{"uid":"u1"}]}`))
	runGitC(t, workdir, "add", ".")
	runGitC(t, workdir, "commit", "-m", "stranded snapshot")

	local := strings.TrimSpace(gitRevParseHeadForTest(t, workdir))
	if local == "" || local == c0 {
		t.Fatalf("test setup: expected a new local commit ahead of remote, local=%q c0=%q", local, c0)
	}

	// Break only the fetch/ls-remote path: url points at a nonexistent bare (ls-remote exits 128),
	// while pushurl stays valid so a regression would visibly deliver the stranded commit.
	gonePath := filepath.Join(t.TempDir(), "gone.git")
	runGitC(t, workdir, "remote", "set-url", "origin", "file://"+gonePath)
	runGitC(t, workdir, "remote", "set-url", "--push", "origin", cloneURL)

	cfg := Config{Endpoint: cloneURL, PushPolicy: PushPolicyCommit}
	err := syncCLIWorkdir(t.Context(), workdir, cloneURL, "main", cfg, CommitContext{Checksum: "c1"}, nil)

	// Fail-safe #1: the ls-remote failure must surface as a non-nil error for the right reason.
	if err == nil {
		t.Fatalf("syncCLIWorkdir() = nil, want ls-remote failure to abort the sync (false-synced regression)")
	}
	if !strings.Contains(err.Error(), "ls-remote") {
		t.Fatalf("syncCLIWorkdir() error = %v, want it wrapped from the git ls-remote failure", err)
	}

	// Fail-safe #2: no bogus delivery. Because the sync aborted before the push, the remote must
	// still sit at C0 -- it must NOT have advanced to the unverified stranded local HEAD.
	if got := bareRef(t, bare, "main"); got != c0 {
		t.Fatalf("remote main = %q, want unchanged %q -- sync pushed a stranded snapshot it could not verify", got, c0)
	}
}

func gitRevParseHeadForTest(t *testing.T, workdir string) string {
	t.Helper()

	cmd := exec.Command("git", "-C", workdir, "rev-parse", "HEAD") //nolint:gosec // G204: test fixture
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	return string(out)
}
