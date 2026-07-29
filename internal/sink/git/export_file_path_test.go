// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestExportFilePath_TrojanPATHNotUsed proves that the git subprocess calls in export_file.go
// (ensureBareHEAD) do not trust an arbitrary inherited PATH. It plants a fake "git" executable
// ahead of the real system git on PATH; if resolution trusted the ambient PATH (as
// exec.CommandContext(ctx, "git", ...) does by default), the trojan would run instead of the
// real binary. It must not.
func TestExportFilePath_TrojanPATHNotUsed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH trojan simulation targets unix-style exec resolution")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	bareDir := t.TempDir()
	runGit(t, "init", "--bare", "-b", "main", bareDir)

	// Plant a trojan "git" that just proves it ran, in a directory placed ahead of the real
	// system git on PATH.
	trojanDir := t.TempDir()
	marker := filepath.Join(trojanDir, "trojan-ran.marker")
	script := "#!/bin/sh\ntouch " + shellQuote(marker) + "\nexit 0\n"
	trojanGit := filepath.Join(trojanDir, "git")
	if err := os.WriteFile(trojanGit, []byte(script), 0o755); err != nil { //nolint:gosec // G306: test fixture binary must be executable
		t.Fatalf("write trojan git: %v", err)
	}

	t.Setenv("PATH", trojanDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cloneURL := "file://" + bareDir
	if err := ensureBareHEAD(t.Context(), cloneURL, "main", nil); err != nil {
		t.Fatalf("ensureBareHEAD() error = %v, want nil (the real system git should run)", err)
	}

	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("trojan git planted earlier on PATH was executed; " +
			"git exec resolution in export_file.go must not trust the ambient PATH")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat marker: %v", statErr)
	}
}

// TestExportFilePath_ResolvesAgainstPinnedPATH proves that the resolved git executable used
// for export_file.go's exec sinks is pinned, not inherited verbatim from the ambient
// environment: even with a hostile/bogus ambient PATH, resolution still finds the real system
// git and never echoes the hostile PATH value back out.
func TestExportFilePath_ResolvesAgainstPinnedPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH pinning simulation targets unix-style exec resolution")
	}

	t.Setenv("PATH", "/nonexistent/evil/path")

	gitPath, err := resolveGitExecutable()
	if err != nil {
		t.Fatalf("resolveGitExecutable() error = %v, want a system git to resolve despite a hostile ambient PATH", err)
	}

	if !filepath.IsAbs(gitPath) {
		t.Fatalf("resolveGitExecutable() = %q, want an absolute path", gitPath)
	}

	if strings.Contains(gitPath, "/nonexistent/evil/path") {
		t.Fatalf("resolveGitExecutable() = %q, appears to have used the ambient PATH", gitPath)
	}
}
