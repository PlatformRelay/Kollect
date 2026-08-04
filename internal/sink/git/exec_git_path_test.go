// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestExecGitPath_TrojanPATHNotUsed proves that gitInWorkdir / gitCloneCmd (exec_git.go)
// do not trust an arbitrary inherited PATH. A fake "git" planted ahead of the real binary
// on PATH must never run — the same SEC-04h guarantee export_file.go already holds.
func TestExecGitPath_TrojanPATHNotUsed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH trojan simulation targets unix-style exec resolution")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	workdir := t.TempDir()
	runGit(t, "init", "-b", "main", workdir)

	trojanDir := t.TempDir()
	marker := filepath.Join(trojanDir, "trojan-ran.marker")
	script := "#!/bin/sh\ntouch " + shellQuote(marker) + "\nexit 0\n"
	trojanGit := filepath.Join(trojanDir, "git")
	if err := os.WriteFile(trojanGit, []byte(script), 0o755); err != nil { //nolint:gosec // G306: test fixture binary must be executable
		t.Fatalf("write trojan git: %v", err)
	}

	t.Setenv("PATH", trojanDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := gitInit(t.Context(), workdir, nil); err != nil {
		t.Fatalf("gitInit() error = %v, want nil (the real system git should run)", err)
	}

	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("trojan git planted earlier on PATH was executed; " +
			"git exec resolution in exec_git.go must not trust the ambient PATH")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat marker: %v", statErr)
	}
}

// TestExecGitPath_CmdUsesAbsoluteBinary proves gitInWorkdir wires an absolute Path so
// Start cannot re-resolve a bare "git" name against the ambient PATH.
func TestExecGitPath_CmdUsesAbsoluteBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH pinning simulation targets unix-style exec resolution")
	}

	t.Setenv("PATH", "/nonexistent/evil/path")

	workdir := t.TempDir()
	cmd := gitInWorkdir(t.Context(), workdir, nil, "status", "--porcelain")
	if cmd == nil {
		t.Fatal("gitInWorkdir() returned nil")
	}
	if cmd.Err != nil {
		t.Fatalf("gitInWorkdir() cmd.Err = %v, want pinned git to resolve despite hostile PATH", cmd.Err)
	}
	if !filepath.IsAbs(cmd.Path) {
		t.Fatalf("gitInWorkdir() Path = %q, want an absolute path (not bare %q)", cmd.Path, "git")
	}
}
