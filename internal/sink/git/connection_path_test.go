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

// TestConnectionPath_TrojanPATHNotUsed proves that connection.go's ls-remote probe does not
// trust an arbitrary inherited PATH. A fake "git" planted ahead of the real binary on PATH
// must never run — the same SEC-04h guarantee export_file.go already holds for ensureBareHEAD.
func TestConnectionPath_TrojanPATHNotUsed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH trojan simulation targets unix-style exec resolution")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remoteDir := createBareRemoteWithMainCommit(t)

	trojanDir := t.TempDir()
	marker := filepath.Join(trojanDir, "trojan-ran.marker")
	script := "#!/bin/sh\ntouch " + shellQuote(marker) + "\nexit 0\n"
	trojanGit := filepath.Join(trojanDir, "git")
	if err := os.WriteFile(trojanGit, []byte(script), 0o755); err != nil { //nolint:gosec // G306: test fixture binary must be executable
		t.Fatalf("write trojan git: %v", err)
	}

	t.Setenv("PATH", trojanDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := Config{Endpoint: "file://" + remoteDir}.withDefaults()
	if err := lsRemoteUncached(t.Context(), cfg, Auth{}); err != nil {
		t.Fatalf("lsRemoteUncached() error = %v, want nil (the real system git should run)", err)
	}

	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("trojan git planted earlier on PATH was executed; " +
			"git exec resolution in connection.go must not trust the ambient PATH")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat marker: %v", statErr)
	}
}

// TestConnectionPath_AvailabilityUsesPinnedPATH proves lsRemote's "is git present?" gate
// consults the pinned PATH (resolveGitExecutable), not ambient LookPath("git"), so a trojan
// alone on PATH cannot make the probe proceed with an unpinned bare "git".
func TestConnectionPath_AvailabilityUsesPinnedPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH pinning simulation targets unix-style exec resolution")
	}

	// Build the fixture while real git is still on PATH, then shrink PATH to trojan-only.
	remoteDir := createBareRemoteWithMainCommit(t)

	trojanDir := t.TempDir()
	marker := filepath.Join(trojanDir, "trojan-ran.marker")
	// Use absolute /usr/bin/touch: with PATH=trojan-only, a bare "touch" is not found and the
	// marker would never appear — a false pass that hides the ambient-LookPath hole.
	script := "#!/bin/sh\n/usr/bin/touch " + shellQuote(marker) + "\nexit 0\n"
	trojanGit := filepath.Join(trojanDir, "git")
	if err := os.WriteFile(trojanGit, []byte(script), 0o755); err != nil { //nolint:gosec // G306: test fixture binary must be executable
		t.Fatalf("write trojan git: %v", err)
	}

	// Ambient PATH has ONLY the trojan — LookPath("git") would succeed and open the hole.
	t.Setenv("PATH", trojanDir)

	cfg := Config{Endpoint: "file://" + remoteDir}
	if err := TestConnection(t.Context(), cfg, Auth{}); err != nil {
		t.Fatalf("TestConnection() error = %v, want nil via pinned system git", err)
	}

	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("trojan git was executed during connection probe; " +
			"lsRemote must resolve git against the pinned PATH, not ambient LookPath")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat marker: %v", statErr)
	}
}
