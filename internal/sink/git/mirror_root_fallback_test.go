// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setUnusableCacheHome points both HOME (darwin) and XDG_CACHE_HOME (unix) at a
// path *under an existing regular file*, so os.UserCacheDir() resolves to a
// candidate whose parent can never be created -- not even by root, because
// mkdir under a regular file is ENOTDIR, not a permission error. This is the
// root-safe way to simulate the read-only-rootfs failure the container hits.
func setUnusableCacheHome(t *testing.T) {
	t.Helper()

	base := t.TempDir()
	afile := filepath.Join(base, "afile")
	if err := os.WriteFile(afile, []byte("regular file, not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(afile): %v", err)
	}

	t.Setenv("HOME", filepath.Join(afile, "home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(afile, "cache"))
}

// DR-FIND-06: when the cache-based default can't be secured (read-only rootfs
// in the shipped container), mirrorRootDir must fall back to the writable
// temp-based path instead of failing the whole export.
func TestMirrorRootDir_readOnlyCacheFallsBackToTempDir(t *testing.T) {
	t.Setenv(envMirrorDir, "")
	// Isolate the temp fallback into a per-test dir (os.TempDir() honors
	// TMPDIR on darwin and linux) so the assertion stays hermetic and never
	// touches or leaks the shared os.TempDir()/kollect-git-mirrors path.
	t.Setenv("TMPDIR", t.TempDir())
	setUnusableCacheHome(t)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v, want fallback to temp dir", err)
	}

	wantTemp := filepath.Join(os.TempDir(), "kollect-git-mirrors")
	if got != wantTemp {
		t.Fatalf("mirrorRootDir() = %q, want temp fallback %q", got, wantTemp)
	}
	if strings.Contains(got, "afile") {
		t.Fatalf("mirrorRootDir() = %q, still under the unusable cache candidate", got)
	}
}

// A writable cache dir must still be preferred -- the fallback only triggers
// when the cache candidate is genuinely unusable.
func TestMirrorRootDir_writableCacheIsPreferred(t *testing.T) {
	t.Setenv(envMirrorDir, "")
	cacheHome := t.TempDir()
	t.Setenv("HOME", cacheHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}

	wantCache := defaultCacheMirrorRoot()
	if got != wantCache {
		t.Fatalf("mirrorRootDir() = %q, want cache path %q", got, wantCache)
	}
	if got == filepath.Join(os.TempDir(), "kollect-git-mirrors") {
		t.Fatalf("mirrorRootDir() fell back to temp %q despite a writable cache", got)
	}
}

// An explicit KOLLECT_GIT_MIRROR_DIR is operator intent and must be honored
// exactly -- never redirected to the temp fallback.
func TestMirrorRootDir_explicitOverrideIsNotRedirected(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "operator-mirrors")
	t.Setenv(envMirrorDir, custom)
	// Even with an unusable cache candidate, the explicit path wins.
	setUnusableCacheHome(t)
	t.Setenv(envMirrorDir, custom)

	got, err := mirrorRootDir()
	if err != nil {
		t.Fatalf("mirrorRootDir() error = %v", err)
	}
	if got != custom {
		t.Fatalf("mirrorRootDir() = %q, want explicit override %q", got, custom)
	}
	if got == filepath.Join(os.TempDir(), "kollect-git-mirrors") {
		t.Fatalf("mirrorRootDir() redirected explicit override to temp %q", got)
	}
}
