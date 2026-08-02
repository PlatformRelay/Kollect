// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMirrorRootDir_envOverrideWins(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-mirrors")
	t.Setenv(envMirrorDir, custom)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}
	if got != custom {
		t.Fatalf("mirrorRootDir() = %q, want %q", got, custom)
	}
}

func TestMirrorRootDir_defaultsAwayFromFixedTempName(t *testing.T) {
	t.Setenv(envMirrorDir, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}

	bareTempDirDefault := filepath.Join(os.TempDir(), "kollect-git-mirrors")
	if got == bareTempDirDefault {
		t.Fatalf("mirrorRootDir() = %q, still the predictable bare-TempDir default", got)
	}
	if got != defaultMirrorRoot() {
		t.Fatalf("mirrorRootDir() = %q, want %q", got, defaultMirrorRoot())
	}
}

func TestMirrorRootDir_createsWithNarrowPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fresh-mirrors")
	t.Setenv(envMirrorDir, root)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}

	info, statErr := os.Stat(got)
	if statErr != nil {
		t.Fatalf("Stat(%q): %v", got, statErr)
	}
	if perm := info.Mode().Perm(); perm != mirrorRootPerm {
		t.Fatalf("mirror root perm = %04o, want %04o", perm, mirrorRootPerm)
	}
}

func TestMirrorRootDir_hostileWorldWritableDirNotAdopted(t *testing.T) {
	hostile := filepath.Join(t.TempDir(), "kollect-git-mirrors")
	if err := os.MkdirAll(hostile, 0o777); err != nil { //nolint:gosec // G301: intentional hostile fixture
		t.Fatalf("MkdirAll(hostile): %v", err)
	}
	// MkdirAll's requested mode is subject to umask; force the write bits so
	// the test exercises the actual hazard regardless of the host's umask.
	if err := os.Chmod(hostile, 0o777); err != nil { //nolint:gosec // G302: intentional hostile fixture
		t.Fatalf("Chmod(hostile): %v", err)
	}
	t.Setenv(envMirrorDir, hostile)

	_, err := mirrorRootDir()
	if err == nil {
		t.Fatal("mirrorRootDir() error = nil, want refusal of world-writable pre-existing dir")
	}
	if !errors.Is(err, errMirrorRootInsecure) {
		t.Fatalf("mirrorRootDir() error = %v, want errMirrorRootInsecure", err)
	}
}

func TestMirrorRootDir_hostileSymlinkNotAdopted(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("MkdirAll(target): %v", err)
	}

	hostile := filepath.Join(parent, "kollect-git-mirrors")
	if err := os.Symlink(target, hostile); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	t.Setenv(envMirrorDir, hostile)

	_, err := mirrorRootDir()
	if err == nil {
		t.Fatal("mirrorRootDir() error = nil, want refusal of a symlinked mirror root")
	}
	if !errors.Is(err, errMirrorRootInsecure) {
		t.Fatalf("mirrorRootDir() error = %v, want errMirrorRootInsecure", err)
	}
}

func TestDefaultMirrorRoot_containerWithoutHomeFallsBackToTempDir(t *testing.T) {
	// When neither HOME nor XDG_CACHE_HOME is set, os.UserCacheDir() errors,
	// so there is no cache candidate at all. Lock in that defaultMirrorRoot()
	// then yields the historical, still-guarded temp path rather than silently
	// producing something else. (In the shipped container HOME *is* set, but
	// its cache path is on the read-only rootfs -- that runtime fallback lives
	// in mirrorRootDir(); see TestMirrorRootDir_readOnlyCacheFallsBackToTempDir.)
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	want := filepath.Join(os.TempDir(), "kollect-git-mirrors")
	if got := defaultMirrorRoot(); got != want {
		t.Fatalf("defaultMirrorRoot() = %q, want %q (container no-HOME fallback)", got, want)
	}
}

func TestMirrorRootDir_hostileRegularFileNotAdopted(t *testing.T) {
	hostile := filepath.Join(t.TempDir(), "kollect-git-mirrors")
	if err := os.WriteFile(hostile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(hostile): %v", err)
	}
	t.Setenv(envMirrorDir, hostile)

	_, err := mirrorRootDir()
	if err == nil {
		t.Fatal("mirrorRootDir() error = nil, want refusal of a pre-existing regular file")
	}
	if !errors.Is(err, errMirrorRootInsecure) {
		t.Fatalf("mirrorRootDir() error = %v, want errMirrorRootInsecure", err)
	}
}

func TestMirrorRootDir_existingNonWritableDirIsReused(t *testing.T) {
	// t.TempDir() dirs are owned by the current user with no group/other
	// write bit (mode varies by platform, e.g. 0700 or 0755) -- safe to reuse.
	dir := t.TempDir()
	t.Setenv(envMirrorDir, dir)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}
	if got != dir {
		t.Fatalf("mirrorRootDir() = %q, want %q", got, dir)
	}
}
