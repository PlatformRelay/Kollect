// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

// Regression guards for the CodeQL error-severity findings that were remediated in this package
// (SEC-01). CodeQL reported 11 alerts, all now state=fixed by real code:
//
//   - go/command-injection in exec_git.go — untrusted branch / workdir / config values flowing into
//     the argv of exec.CommandContext ("git", ...). Remediated by validating every untrusted input
//     (ValidateGitRef, validateGitWorkdir, validateGitConfigValue, validateObjectPath) BEFORE the
//     command is built, invoking git via an argv slice (no shell), and terminating options with "--".
//   - go/command-injection + go/path-injection in export_file.go — a file:// clone URL flowing into
//     `git --git-dir <bareDir>` (ensureBareHEAD) and object paths flowing into on-disk writes.
//     Remediated by parseFileGitBarePath / objectPathInWorkdir containment before any exec or write.
//
// The invariant these tests lock in: the exec-invoking helpers reject hostile input BEFORE reaching
// the exec sink, so a future refactor that drops a validator turns these RED instead of silently
// reintroducing the injection. Most helpers already had guards (see exec_git_test.go,
// exec_git_additional_test.go, validate_test.go); the cases below close the remaining flagged-sink
// paths that lacked a direct guard.

import "testing"

// gitInit builds `git -C <workdir> init`; workdir is the untrusted input reaching the exec sink.
func TestGitInit_rejectsInvalidWorkdir(t *testing.T) {
	t.Parallel()

	if err := gitInit(t.Context(), "bad\x00dir", nil); err == nil {
		t.Fatal("expected error for workdir with null byte")
	}
	if err := gitInit(t.Context(), "", nil); err == nil {
		t.Fatal("expected error for empty workdir")
	}
}

// gitFetchShallow builds `git -C <workdir> fetch origin <branch>`; branch reaches the exec sink.
func TestGitFetchShallow_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	if err := gitFetchShallow(t.Context(), t.TempDir(), "--upload-pack=evil", 1, nil); err == nil {
		t.Fatal("expected error for flag-like fetch branch")
	}
	if err := gitFetchShallow(t.Context(), t.TempDir(), "; rm -rf /", 0, nil); err == nil {
		t.Fatal("expected error for shell-metacharacter fetch branch")
	}
}

// ensureBareHEAD resolves a file:// clone URL to a bare dir that flows into
// `git --git-dir <bareDir> symbolic-ref ...` (the export_file.go command-injection sink). A hostile
// clone URL must be rejected by parseFileGitBarePath before the command is built.
func TestEnsureBareHEAD_rejectsMaliciousCloneURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cloneURL string
	}{
		{name: "flag-like file path", cloneURL: "file://--upload-pack=evil"},
		{name: "null byte in path", cloneURL: "file:///tmp/repo%00.git"},
		{name: "empty file path", cloneURL: "file://"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ensureBareHEAD(t.Context(), tc.cloneURL, "main", nil); err == nil {
				t.Fatalf("ensureBareHEAD(%q) = nil, want error", tc.cloneURL)
			}
		})
	}
}
