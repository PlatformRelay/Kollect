// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"
)

func TestGitClone_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := gitClone(t.Context(), dir, "file:///tmp/repo.git", "--upload-pack=evil", 1)
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestGitClone_rejectsMaliciousCloneURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := gitClone(t.Context(), dir, "file://--upload-pack=evil", "main", 1)
	if err == nil {
		t.Fatal("expected error for malicious clone URL")
	}
}

func TestGitCheckoutNewBranch_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := gitCheckoutNewBranch(t.Context(), dir, "; rm -rf /")
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestGitAddPath_rejectsTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := gitAddPath(t.Context(), dir, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestGitPushOrigin_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := gitPushOrigin(t.Context(), dir, false, "-B evil")
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestGitRemoteAddOrigin_rejectsMaliciousCloneURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := gitRemoteAddOrigin(t.Context(), dir, "--upload-pack=evil")
	if err == nil {
		t.Fatal("expected error for malicious clone URL")
	}
}

func TestValidateGitWorkdir_rejectsInvalidCharacters(t *testing.T) {
	t.Parallel()

	if _, err := validateGitWorkdir("/tmp/work\x00dir"); err == nil {
		t.Fatal("expected rejection of workdir with null byte")
	}
}

func TestValidateGitConfigValue_rejectsFlagLikeAuthor(t *testing.T) {
	t.Parallel()

	if err := validateGitConfigValue("--upload-pack=evil"); err == nil {
		t.Fatal("expected rejection of flag-like config value")
	}
}
