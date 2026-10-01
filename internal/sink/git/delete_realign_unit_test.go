// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"path/filepath"
	"strings"
	"testing"
)

// MR-05 unit lock for realignDivergedDirectBranch: in direct mode a local
// branch whose stranded commit no longer fast-forwards onto the fetched remote
// tip is reset to that tip; one that still fast-forwards is kept.
func TestRealignDivergedDirectBranch(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)
	direct := exportRequest{cloneURL: remote, cloneBranch: "main", pushBranch: "main", objectPath: "inventory/team-a/inv.json"}

	// Stranded local commit that still fast-forwards: kept.
	mustWriteFile(t, filepath.Join(work, "stranded.txt"), []byte("stranded\n"))
	runGitC(t, work, "add", "stranded.txt")
	runGitC(t, work, "commit", "-q", "-m", "stranded")
	stranded := gitOutput(t, work, "rev-parse", "HEAD")

	if err := realignDivergedDirectBranch(t.Context(), work, direct, nil); err != nil {
		t.Fatalf("realign (fast-forward): %v", err)
	}
	if head := gitOutput(t, work, "rev-parse", "HEAD"); head != stranded {
		t.Fatalf("fast-forwardable stranded commit was dropped: HEAD=%s, want %s", head, stranded)
	}

	// The remote advances independently: the stranded commit diverges and the
	// local branch is reset to the fetched remote tip.
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "clone", "-q", remote, other)
	runGitC(t, other, "config", "user.name", "other")
	runGitC(t, other, "config", "user.email", "other@example.com")
	mustWriteFile(t, filepath.Join(other, "foreign.txt"), []byte("foreign\n"))
	runGitC(t, other, "add", "foreign.txt")
	runGitC(t, other, "commit", "-q", "-m", "foreign")
	runGitC(t, other, "push", "-q", "origin", "main")
	runGitC(t, work, "fetch", "-q", "origin", "main")
	remoteTip := gitOutput(t, work, "rev-parse", "refs/remotes/origin/main")

	if err := realignDivergedDirectBranch(t.Context(), work, direct, nil); err != nil {
		t.Fatalf("realign (diverged): %v", err)
	}
	if head := gitOutput(t, work, "rev-parse", "HEAD"); head != remoteTip {
		t.Fatalf("diverged branch not reset: HEAD=%s, want remote tip %s", head, remoteTip)
	}

	// Branch mode is never realigned: its stranded tips have their own gate.
	branchMode := direct
	branchMode.pushBranch = "kollect/team-a/inv"
	mustWriteFile(t, filepath.Join(work, "local.txt"), []byte("local\n"))
	runGitC(t, work, "add", "local.txt")
	runGitC(t, work, "commit", "-q", "-m", "local")
	local := gitOutput(t, work, "rev-parse", "HEAD")
	if err := realignDivergedDirectBranch(t.Context(), work, branchMode, nil); err != nil {
		t.Fatalf("realign (branch mode): %v", err)
	}
	if head := gitOutput(t, work, "rev-parse", "HEAD"); head != local {
		t.Fatalf("branch mode must not realign: HEAD=%s, want %s", head, local)
	}

	// No remote-tracking ref (empty remote): nothing to realign against.
	noRemote := direct
	noRemote.cloneBranch, noRemote.pushBranch = "absent", "absent"
	if err := realignDivergedDirectBranch(t.Context(), work, noRemote, nil); err != nil {
		t.Fatalf("realign (no remote ref): %v", err)
	}
}

func TestGitResetHardTo_RejectsNonHash(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	work := initCLIFixtureMirror(t, remote)

	err := gitResetHardTo(t.Context(), work, "origin/main", nil)
	if err == nil || !strings.Contains(err.Error(), "not a commit hash") {
		t.Fatalf("gitResetHardTo(ref) = %v, want a not-a-commit-hash error", err)
	}

	if _, exists, err := gitRevParse(t.Context(), work, "refs/heads/does-not-exist", nil); err != nil || exists {
		t.Fatalf("gitRevParse(missing) = exists %v err %v, want false/nil", exists, err)
	}
}
