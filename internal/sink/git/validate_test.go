// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateObjectPath_rejectsTraversal(t *testing.T) {
	t.Parallel()

	cases := []string{
		"../../../etc/passwd",
		"inventory/../../outside.json",
		"/etc/passwd",
		"inventory/latest.json\x00.evil",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if _, err := validateObjectPath(tc); err == nil {
				t.Fatalf("validateObjectPath(%q) = nil, want error", tc)
			}
		})
	}
}

func TestValidateObjectPath_acceptsSafePaths(t *testing.T) {
	t.Parallel()

	got, err := validateObjectPath("inventory/team-a/deployments.json")
	if err != nil {
		t.Fatalf("validateObjectPath() error = %v", err)
	}

	if got != "inventory/team-a/deployments.json" {
		t.Fatalf("got %q", got)
	}
}

func TestObjectPathInWorkdir_containedInWorkdir(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	abs, rel, err := objectPathInWorkdir(workdir, "inventory/test.json")
	if err != nil {
		t.Fatalf("objectPathInWorkdir() error = %v", err)
	}

	if rel != "inventory/test.json" {
		t.Fatalf("rel = %q", rel)
	}

	if !strings.HasPrefix(abs, workdir) {
		t.Fatalf("abs %q not under workdir %q", abs, workdir)
	}
}

func TestObjectPathInWorkdir_rejectsEscape(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	if _, _, err := objectPathInWorkdir(workdir, "../../../etc/passwd"); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestValidateGitRef_rejectsInjection(t *testing.T) {
	t.Parallel()

	cases := []string{
		"; rm -rf /",
		"--help",
		"-B evil",
		"branch; rm -rf /",
		"refs/heads/main",
		"branch name",
		"branch..name",
		".hidden",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if err := ValidateGitRef(tc); err == nil {
				t.Fatalf("ValidateGitRef(%q) = nil, want error", tc)
			}
		})
	}
}

func TestValidateGitRef_acceptsFeatureBranch(t *testing.T) {
	t.Parallel()

	cases := []string{
		"main",
		"develop",
		"kollect/team-a/inventory",
		"release-1.2.3",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if err := ValidateGitRef(tc); err != nil {
				t.Fatalf("ValidateGitRef(%q) error = %v", tc, err)
			}
		})
	}
}

func TestValidateCloneURL_rejectsFlagLikeURL(t *testing.T) {
	t.Parallel()

	if err := validateCloneURL("--upload-pack=evil"); err == nil {
		t.Fatal("expected rejection of flag-like URL")
	}
}

func TestValidateCloneURL_rejectsMaliciousFileURL(t *testing.T) {
	t.Parallel()

	cases := []string{
		"file://--upload-pack=evil",
		"file:///tmp/repo.git\x00.evil",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if err := validateCloneURL(tc); err == nil {
				t.Fatalf("validateCloneURL(%q) = nil, want error", tc)
			}
		})
	}
}

func TestParseFileGitBarePath_resolvesAndRejects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	if err := os.MkdirAll(bare, 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := parseFileGitBarePath("file://" + bare)
	if err != nil {
		t.Fatalf("parseFileGitBarePath() error = %v", err)
	}

	want, err := filepath.Abs(bare)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("parseFileGitBarePath() = %q, want %q", got, want)
	}

	if _, err := parseFileGitBarePath("file://--upload-pack=evil"); err == nil {
		t.Fatal("expected rejection of flag-like file path")
	}
}

func TestParseFileGitBarePath_errorBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cloneURL string
	}{
		{name: "non-file scheme", cloneURL: "https://example.com/repo.git"},
		{name: "empty path", cloneURL: "file://"},
		{name: "null byte", cloneURL: "file:///tmp/repo%00.git"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseFileGitBarePath(tc.cloneURL); err == nil {
				t.Fatalf("parseFileGitBarePath(%q) expected error", tc.cloneURL)
			}
		})
	}
}

func TestValidateGitConfigValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "Bot Name"},
		{name: "empty", value: "", wantErr: true},
		{name: "flag-like", value: "-x", wantErr: true},
		{name: "null byte", value: "bad\x00value", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateGitConfigValue(tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("validateGitConfigValue(%q) expected error", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateGitConfigValue(%q) unexpected error: %v", tc.value, err)
			}
		})
	}
}

func TestValidateGitWorkdir(t *testing.T) {
	t.Parallel()

	if _, err := validateGitWorkdir(""); err == nil {
		t.Fatal("expected error for empty workdir")
	}
	if _, err := validateGitWorkdir("bad\ndir"); err == nil {
		t.Fatal("expected error for newline in workdir")
	}

	dir := t.TempDir()
	got, err := validateGitWorkdir(dir)
	if err != nil {
		t.Fatalf("validateGitWorkdir() error = %v", err)
	}
	if got == "" {
		t.Fatal("expected resolved workdir path")
	}
}

func TestCanonicalCloneURL_nonFilePassesThrough(t *testing.T) {
	t.Parallel()

	got, err := canonicalCloneURL("https://example.com/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/repo.git" {
		t.Fatalf("canonicalCloneURL() = %q", got)
	}
}

func TestCanonicalCloneURL_propagatesValidationError(t *testing.T) {
	t.Parallel()

	if _, err := canonicalCloneURL("file://--upload-pack=evil"); err == nil {
		t.Fatal("expected error for flag-like clone URL")
	}
}

func TestValidateExportFiles_errorBranches(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "https://example.com/repo.git"}

	t.Run("no files", func(t *testing.T) {
		t.Parallel()

		if _, _, err := validateExportFiles(cfg, nil, nil); err == nil {
			t.Fatal("expected error for empty file set")
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		t.Parallel()

		_, _, err := validateExportFiles(cfg, []FileEntry{{Path: "a.json", Data: nil}}, nil)
		if err == nil {
			t.Fatal("expected error for empty payload")
		}
	})

	t.Run("duplicate path", func(t *testing.T) {
		t.Parallel()

		files := []FileEntry{
			{Path: "a.json", Data: []byte("x")},
			{Path: "a.json", Data: []byte("y")},
		}
		if _, _, err := validateExportFiles(cfg, files, nil); err == nil {
			t.Fatal("expected error for duplicate object path")
		}
	})

	t.Run("invalid clone branch", func(t *testing.T) {
		t.Parallel()

		files := []FileEntry{{Path: "a.json", Data: []byte("x")}}
		_, _, err := validateExportFiles(cfg, files, &BranchSpec{CloneBranch: "-bad"})
		if err == nil {
			t.Fatal("expected error for invalid clone branch")
		}
	})

	t.Run("invalid push branch", func(t *testing.T) {
		t.Parallel()

		files := []FileEntry{{Path: "a.json", Data: []byte("x")}}
		_, _, err := validateExportFiles(cfg, files, &BranchSpec{CloneBranch: "main", PushBranch: "-bad"})
		if err == nil {
			t.Fatal("expected error for invalid push branch")
		}
	})
}

func TestValidateGitCommitMessage(t *testing.T) {
	t.Parallel()

	if err := validateGitCommitMessage("chore: export inventory"); err != nil {
		t.Fatalf("unexpected error for clean message: %v", err)
	}
	if err := validateGitCommitMessage("bad\x00message"); err == nil {
		t.Fatal("expected error for null byte in commit message")
	}
}

func TestCanonicalCloneURL_normalizesFileURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bare := filepath.Join(dir, "nested", "repo.git")
	if err := os.MkdirAll(bare, 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := canonicalCloneURL("file://" + filepath.Join(dir, "nested", "..", "nested", "repo.git"))
	if err != nil {
		t.Fatalf("canonicalCloneURL() error = %v", err)
	}

	want, err := canonicalCloneURL("file://" + bare)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("canonicalCloneURL() = %q, want %q", got, want)
	}
}

func TestEnsureBareHEAD_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	err := ensureBareHEAD(t.Context(), "file:///tmp/repo.git", "; rm -rf /", nil)
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestCloneOrInitCLI_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := cloneOrInitCLI(t.Context(), dir, "file:///tmp/repo.git", "--upload-pack=evil", 1, nil)
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestCloneOrInitCLI_rejectsMaliciousCloneURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := cloneOrInitCLI(t.Context(), dir, "file://--upload-pack=evil", "main", 1, nil)
	if err == nil {
		t.Fatal("expected error for malicious clone URL")
	}
}

func TestExportWithBranch_rejectsMaliciousObjectPath(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/repo.git"}
	err := ExportWithBranch(t.Context(), cfg, Auth{}, []byte("{}"), "../../../etc/passwd", nil, CommitContext{})
	if err == nil {
		t.Fatal("expected error for malicious object path")
	}
}

func TestExportWithBranch_rejectsMaliciousBranch(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/repo.git"}
	err := ExportWithBranch(t.Context(), cfg, Auth{}, []byte("{}"), "inventory/test.json", &BranchSpec{
		PushBranch: "; rm -rf /",
	}, CommitContext{})
	if err == nil {
		t.Fatal("expected error for malicious branch")
	}
}

func TestExportViaCLI_rejectsTraversalBeforeWrite(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	outside := filepath.Join(workdir, "outside.json")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	bare := filepath.Join(workdir, "repo.git")
	err := exportViaCLI(
		t.Context(),
		Config{Endpoint: "file://" + bare}.withDefaults(),
		Auth{},
		"file://"+bare,
		"main",
		"main",
		[]FileEntry{{Path: "../outside.json", Data: []byte(`{"x":1}`)}},
		CommitContext{},
	)
	if err == nil {
		t.Fatal("expected traversal rejection")
	}

	data, readErr := os.ReadFile(outside) //nolint:gosec // G304: test fixture path from t.TempDir
	if readErr != nil {
		t.Fatal(readErr)
	}

	if string(data) != "secret" {
		t.Fatalf("outside file was modified: %q", data)
	}
}
