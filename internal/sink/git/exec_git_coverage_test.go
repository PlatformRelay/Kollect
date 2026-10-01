// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"

	"github.com/go-git/go-git/v5"
)

// Every git CLI helper rejects an invalid workdir (and invalid refs) before it
// shells out, so a caller can never point them at an unvalidated path.
func TestExecGitHelpers_RejectInvalidInputs(t *testing.T) {
	t.Parallel()

	const badWorkdir = "bad\x00dir"

	if _, err := gitRefExists(t.Context(), badWorkdir, "refs/heads/main", nil); err == nil {
		t.Error("gitRefExists must reject an invalid workdir")
	}
	if err := gitCheckoutPushBranch(t.Context(), t.TempDir(), "main", "bad ref", nil); err == nil {
		t.Error("gitCheckoutPushBranch must reject an invalid push branch")
	}
	if err := gitCheckoutPushBranch(t.Context(), t.TempDir(), "bad ref", "feature", nil); err == nil {
		t.Error("gitCheckoutPushBranch must reject an invalid clone branch")
	}
	if err := gitCheckoutPushBranch(t.Context(), badWorkdir, "main", "feature", nil); err == nil {
		t.Error("gitCheckoutPushBranch must reject an invalid workdir")
	}
	if err := gitAddPaths(t.Context(), badWorkdir, []string{"inventory/team-a/inv.json"}, nil); err == nil {
		t.Error("gitAddPaths must reject an invalid workdir")
	}
	if err := gitResetHard(t.Context(), badWorkdir, nil); err == nil {
		t.Error("gitResetHard must reject an invalid workdir")
	}
	if err := gitCleanFd(t.Context(), badWorkdir, nil); err == nil {
		t.Error("gitCleanFd must reject an invalid workdir")
	}
	if _, _, err := gitRefHash(t.Context(), t.TempDir(), "bad ref", nil); err == nil {
		t.Error("gitRefHash must reject an invalid branch")
	}
	if _, _, err := gitRefHash(t.Context(), badWorkdir, "main", nil); err == nil {
		t.Error("gitRefHash must reject an invalid workdir")
	}
	if _, err := gitHeadHash(t.Context(), badWorkdir, nil); err == nil {
		t.Error("gitHeadHash must reject an invalid workdir")
	}
	if _, err := gitIsAncestorOfHead(t.Context(), badWorkdir, "0123456789012345678901234567890123456789", nil); err == nil {
		t.Error("gitIsAncestorOfHead must reject an invalid workdir")
	}
}

// gitResetHard is a no-op on an unborn HEAD (nothing committed to reset to),
// and gitHeadHash reports the rev-parse failure rather than returning an empty
// hash.
func TestGitResetHardAndHeadHash_UnbornHead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	if err := gitResetHard(t.Context(), dir, nil); err != nil {
		t.Fatalf("gitResetHard on unborn HEAD = %v, want nil", err)
	}
	if _, err := gitHeadHash(t.Context(), dir, nil); err == nil {
		t.Fatal("gitHeadHash on unborn HEAD must error")
	}
}
