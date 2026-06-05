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

func TestExportWithBranch_rejectsMaliciousObjectPath(t *testing.T) {
	t.Parallel()

	cfg := Config{Endpoint: "file:///tmp/repo.git"}
	err := ExportWithBranch(t.Context(), cfg, Auth{}, []byte("{}"), "../../../etc/passwd", nil, CommitContext{})
	if err == nil {
		t.Fatal("expected error for malicious object path")
	}

	if !strings.Contains(err.Error(), "object path") {
		t.Fatalf("error = %v, want object path validation failure", err)
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

	if !strings.Contains(err.Error(), "branch") {
		t.Fatalf("error = %v, want branch validation failure", err)
	}
}

func TestExportFileRemote_rejectsTraversalBeforeWrite(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	outside := filepath.Join(workdir, "outside.json")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	bare := filepath.Join(workdir, "repo.git")
	err := exportFileRemote(
		t.Context(),
		Config{Endpoint: "file://" + bare}.withDefaults(),
		"file://"+bare,
		"main",
		"main",
		[]byte(`{"x":1}`),
		"../outside.json",
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
