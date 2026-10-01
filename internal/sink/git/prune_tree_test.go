// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

func TestRemoveBillyOrphans_RemovesOnlyUnwrittenFiles(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "inventory/team-a/keep.json", "{}")
	mustWriteBillyFile(t, fs, "inventory/team-a/stale.json", "{}")
	mustWriteBillyFile(t, fs, "inventory/team-b/current.json", "{}")
	mustWriteBillyFile(t, fs, "inventory/team-b/old.json", "{}")

	written := []string{
		"inventory/team-a/keep.json",
		"inventory/team-b/current.json",
	}

	if err := removeBillyOrphans(fs, written); err != nil {
		t.Fatalf("removeBillyOrphans() error = %v", err)
	}
	assertBillyExists(t, fs, "inventory/team-a/keep.json", true)
	assertBillyExists(t, fs, "inventory/team-b/current.json", true)
	assertBillyExists(t, fs, "inventory/team-a/stale.json", false)
	assertBillyExists(t, fs, "inventory/team-b/old.json", false)
}

func TestRemoveBillyOrphans_DropsKindDirectoryThatLostItsLastFile(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/api.yaml", "a")
	mustWriteBillyFile(t, fs, "prod/team-a/Service/web.yaml", "b")
	mustWriteBillyFile(t, fs, "prod/team-b/Deployment/other.yaml", "c")

	written := []string{"prod/team-a/Service/web.yaml"}
	if err := removeBillyOrphans(fs, written); err != nil {
		t.Fatalf("removeBillyOrphans() error = %v", err)
	}

	assertBillyExists(t, fs, "prod/team-a/Deployment/api.yaml", false)
	assertBillyExists(t, fs, "prod/team-a/Service/web.yaml", true)
	assertBillyExists(t, fs, "prod/team-b/Deployment/other.yaml", true)
}

func TestRemoveBillyOrphans_ShallowNeighborTreeSurvives(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "inventory/team-a/keep.json", "{}")
	mustWriteBillyFile(t, fs, "inventory/team-b/other.json", "{}")

	if err := removeBillyOrphans(fs, []string{"inventory/team-a/keep.json"}); err != nil {
		t.Fatalf("removeBillyOrphans() error = %v", err)
	}

	assertBillyExists(t, fs, "inventory/team-a/keep.json", true)
	assertBillyExists(t, fs, "inventory/team-b/other.json", true)
}

func TestRemoveDiskOrphans_DropsKindDirectoryThatLostItsLastFile(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "prod/team-a/Deployment/api.yaml", "a")
	mustWriteDiskFile(t, workdir, "prod/team-a/Service/web.yaml", "b")
	mustWriteDiskFile(t, workdir, "prod/team-b/Deployment/other.yaml", "c")

	if err := removeDiskOrphans(workdir, []string{"prod/team-a/Service/web.yaml"}); err != nil {
		t.Fatalf("removeDiskOrphans() error = %v", err)
	}

	assertDiskExists(t, workdir, "prod/team-a/Deployment/api.yaml", false)
	assertDiskExists(t, workdir, "prod/team-a/Service/web.yaml", true)
	assertDiskExists(t, workdir, "prod/team-b/Deployment/other.yaml", true)
}

func TestRemoveBillyOrphans_MissingManagedDirIgnored(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	written := []string{
		"inventory/team-a/keep.json",
		"inventory/missing/keep.json",
	}

	mustWriteBillyFile(t, fs, "inventory/team-a/keep.json", "{}")
	if err := removeBillyOrphans(fs, written); err != nil {
		t.Fatalf("removeBillyOrphans() error = %v", err)
	}
}

// TestRemoveBillyOrphans_SubdirectoriesSurvive guards the safety property that prune
// only deletes stale *files* in a managed directory and never descends into or removes
// nested subdirectories (which belong to other, separately-managed inventories).
func TestRemoveBillyOrphans_SubdirectoriesSurvive(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "inventory/team-a/keep.json", "{}")
	mustWriteBillyFile(t, fs, "inventory/team-a/stale.json", "{}")
	// A nested subdirectory under the managed dir, populated by another inventory.
	mustWriteBillyFile(t, fs, "inventory/team-a/nested/child.json", "{}")

	written := []string{"inventory/team-a/keep.json"}

	if err := removeBillyOrphans(fs, written); err != nil {
		t.Fatalf("removeBillyOrphans() error = %v", err)
	}

	assertBillyExists(t, fs, "inventory/team-a/keep.json", true)
	assertBillyExists(t, fs, "inventory/team-a/stale.json", false)
	// The subdirectory and its contents must be untouched by prune.
	assertBillyExists(t, fs, "inventory/team-a/nested/child.json", true)
}

// TestRemoveDiskOrphans_SubdirectoriesSurvive is the CLI-engine analogue: a nested
// subdirectory in a managed dir must survive prune of stale sibling files.
func TestRemoveDiskOrphans_SubdirectoriesSurvive(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "inventory/team-a/keep.json", "{}")
	mustWriteDiskFile(t, workdir, "inventory/team-a/stale.json", "{}")
	mustWriteDiskFile(t, workdir, "inventory/team-a/nested/child.json", "{}")

	written := []string{"inventory/team-a/keep.json"}

	if err := removeDiskOrphans(workdir, written); err != nil {
		t.Fatalf("removeDiskOrphans() error = %v", err)
	}

	assertDiskExists(t, workdir, "inventory/team-a/keep.json", true)
	assertDiskExists(t, workdir, "inventory/team-a/stale.json", false)
	assertDiskExists(t, workdir, "inventory/team-a/nested/child.json", true)
}

func TestRemoveDiskOrphans_RemovesOnlyUnwrittenFiles(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "inventory/team-a/keep.json", "{}")
	mustWriteDiskFile(t, workdir, "inventory/team-a/stale.json", "{}")
	mustWriteDiskFile(t, workdir, "inventory/team-b/current.json", "{}")

	written := []string{
		"inventory/team-a/keep.json",
		"inventory/team-b/current.json",
	}
	if err := removeDiskOrphans(workdir, written); err != nil {
		t.Fatalf("removeDiskOrphans() error = %v", err)
	}

	assertDiskExists(t, workdir, "inventory/team-a/keep.json", true)
	assertDiskExists(t, workdir, "inventory/team-b/current.json", true)
	assertDiskExists(t, workdir, "inventory/team-a/stale.json", false)
}

func TestRemoveDiskOrphans_MissingManagedDirIgnored(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "inventory/team-a/keep.json", "{}")

	written := []string{
		"inventory/team-a/keep.json",
		"inventory/missing/keep.json",
	}
	if err := removeDiskOrphans(workdir, written); err != nil {
		t.Fatalf("removeDiskOrphans() error = %v", err)
	}
}

func mustWriteBillyFile(t *testing.T, fs billy.Filesystem, rel, content string) {
	t.Helper()

	if err := fs.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", rel, err)
	}
	f, err := fs.Create(rel)
	if err != nil {
		t.Fatalf("Create(%q): %v", rel, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatalf("Write(%q): %v", rel, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(%q): %v", rel, err)
	}
}

func assertBillyExists(t *testing.T, fs billy.Filesystem, rel string, want bool) {
	t.Helper()

	_, err := fs.Stat(rel)
	if want && err != nil {
		t.Fatalf("Stat(%q) error = %v, want file present", rel, err)
	}
	if !want && !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) error = %v, want not-exist", rel, err)
	}
}

func mustWriteDiskFile(t *testing.T, workdir, rel, content string) {
	t.Helper()

	full := filepath.Join(workdir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", rel, err)
	}
}

func assertDiskExists(t *testing.T, workdir, rel string, want bool) {
	t.Helper()

	_, err := os.Stat(filepath.Join(workdir, filepath.FromSlash(rel)))
	if want && err != nil {
		t.Fatalf("Stat(%q) error = %v, want file present", rel, err)
	}
	if !want && !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) error = %v, want not-exist", rel, err)
	}
}

func TestPathDepth_RootAndDotAreZero(t *testing.T) {
	t.Parallel()

	if pathDepth(".") != 0 || pathDepth("/") != 0 || pathDepth("prod/team-a/Deployment") != 3 {
		t.Fatalf("pathDepth(.)=%d pathDepth(/)=%d pathDepth(kind)=%d", pathDepth("."), pathDepth("/"), pathDepth("prod/team-a/Deployment"))
	}
}

func TestRemoveBillyOrphans_MissingKindParentIsNotAnError(t *testing.T) {
	t.Parallel()

	if err := removeBillyOrphans(memfs.New(), []string{"prod/team-a/Deployment/api.yaml"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDiskOrphans_MissingKindParentIsNotAnError(t *testing.T) {
	t.Parallel()

	if err := removeDiskOrphans(t.TempDir(), []string{"prod/team-a/Deployment/api.yaml"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDiskOrphans_FileWhereParentDirBelongs(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "prod/team-a", "not-a-dir")

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Deployment/api.yaml"})
	if err == nil || !strings.Contains(err.Error(), "prune read dir") {
		t.Fatalf("error = %v", err)
	}
}

func TestRemoveBillyOrphans_SecondKindSharesParent(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/api.yaml", "a")
	mustWriteBillyFile(t, fs, "prod/team-a/Service/web.yaml", "b")
	mustWriteBillyFile(t, fs, "prod/team-a/ConfigMap/old.yaml", "c")

	err := removeBillyOrphans(fs, []string{
		"prod/team-a/Deployment/api.yaml",
		"prod/team-a/Service/web.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}

	assertBillyExists(t, fs, "prod/team-a/ConfigMap/old.yaml", false)
	assertBillyExists(t, fs, "prod/team-a/Deployment/api.yaml", true)
	assertBillyExists(t, fs, "prod/team-a/Service/web.yaml", true)
}

func TestRemoveDiskOrphans_RemoveDenied(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "prod/team-a/Deployment/api.yaml", "a")
	dir := filepath.Join(workdir, "prod", "team-a", "Deployment")
	if err := os.Chmod(dir, 0o555); err != nil { //nolint:gosec // G302: 0555 is the write denial this test is proving.
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o750) //nolint:gosec // G302: restore the temp dir so the test cleanup can delete it.
	})

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Service/web.yaml"})
	if err == nil || !strings.Contains(err.Error(), "prune remove") {
		t.Fatalf("error = %v", err)
	}
}

func TestRemoveDiskOrphans_UnreadableSiblingDir(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "prod/team-a/Deployment/api.yaml", "a")
	mustWriteDiskFile(t, workdir, "prod/team-a/ConfigMap/old.yaml", "c")
	dir := filepath.Join(workdir, "prod", "team-a", "ConfigMap")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o750) //nolint:gosec // G302: restore the temp dir so the test cleanup can delete it.
	})

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Deployment/api.yaml"})
	if err == nil || !strings.Contains(err.Error(), "prune read dir") {
		t.Fatalf("error = %v", err)
	}
}
