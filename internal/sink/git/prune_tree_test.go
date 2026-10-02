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

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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

	if err := removeBillyOrphans(fs, written, ""); err != nil {
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
	if err := removeBillyOrphans(fs, written, kollectdevv1alpha1.DefaultLayoutPathTemplate); err != nil {
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

	if err := removeBillyOrphans(fs, []string{"inventory/team-a/keep.json"}, kollectdevv1alpha1.DefaultLayoutPathTemplate); err != nil {
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

	if err := removeDiskOrphans(workdir, []string{"prod/team-a/Service/web.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate); err != nil {
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
	if err := removeBillyOrphans(fs, written, ""); err != nil {
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

	if err := removeBillyOrphans(fs, written, ""); err != nil {
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

	if err := removeDiskOrphans(workdir, written, ""); err != nil {
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
	if err := removeDiskOrphans(workdir, written, ""); err != nil {
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
	if err := removeDiskOrphans(workdir, written, ""); err != nil {
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

	if err := removeBillyOrphans(memfs.New(), []string{"prod/team-a/Deployment/api.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDiskOrphans_MissingKindParentIsNotAnError(t *testing.T) {
	t.Parallel()

	if err := removeDiskOrphans(t.TempDir(), []string{"prod/team-a/Deployment/api.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDiskOrphans_FileWhereParentDirBelongs(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "prod/team-a", "not-a-dir")

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Deployment/api.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate)
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
	}, kollectdevv1alpha1.DefaultLayoutPathTemplate)
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

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Service/web.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate)
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

	err := removeDiskOrphans(workdir, []string{"prod/team-a/Deployment/api.yaml"}, kollectdevv1alpha1.DefaultLayoutPathTemplate)
	if err == nil || !strings.Contains(err.Error(), "prune read dir") {
		t.Fatalf("error = %v", err)
	}
}

func TestKindDirectoryDepth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		template string
		want     int
	}{
		{template: "", want: 0},
		{template: ".", want: 0},
		{template: "{kind}", want: 0},
		{template: kollectdevv1alpha1.DefaultLayoutPathTemplate, want: 3},
		{template: "exports/{namespace}/{sourceNamespace}/{sourceName}{extension}", want: 0},
		{template: "clusters/{cluster}/{group}/{kind}/{sourceName}{extension}", want: 4},
		{template: "{group}/{kind}/{sourceName}{extension}", want: 2},
		{template: "collide/{kind}{extension}", want: 0},
		{template: "{kind}/{sourceName}{extension}", want: 1},
		{template: "{cluster}/{sourceNamespace}/pre-{kind}/{sourceName}{extension}", want: 0},
	}
	for _, tc := range cases {
		if got := kindDirectoryDepth(tc.template); got != tc.want {
			t.Errorf("kindDirectoryDepth(%q) = %d, want %d", tc.template, got, tc.want)
		}
	}
}

func TestRemoveBillyOrphans_DeepPrefixNeighborSurvives(t *testing.T) {
	t.Parallel()

	for _, tmpl := range []string{
		"",
		"exports/{namespace}/{sourceNamespace}/{sourceName}{extension}",
	} {
		fs := memfs.New()
		mustWriteBillyFile(t, fs, "exports/inventory/team-a/keep.json", "{}")
		mustWriteBillyFile(t, fs, "exports/inventory/team-b/other.json", "{}")

		if err := removeBillyOrphans(fs, []string{"exports/inventory/team-a/keep.json"}, tmpl); err != nil {
			t.Fatalf("template %q: %v", tmpl, err)
		}

		assertBillyExists(t, fs, "exports/inventory/team-a/keep.json", true)
		assertBillyExists(t, fs, "exports/inventory/team-b/other.json", true)
	}
}

func TestRemoveDiskOrphans_DeepPrefixNeighborSurvives(t *testing.T) {
	t.Parallel()

	for _, tmpl := range []string{
		"",
		"exports/{namespace}/{sourceNamespace}/{sourceName}{extension}",
	} {
		workdir := t.TempDir()
		mustWriteDiskFile(t, workdir, "exports/inventory/team-a/keep.json", "{}")
		mustWriteDiskFile(t, workdir, "exports/inventory/team-b/other.json", "{}")

		if err := removeDiskOrphans(workdir, []string{"exports/inventory/team-a/keep.json"}, tmpl); err != nil {
			t.Fatalf("template %q: %v", tmpl, err)
		}

		assertDiskExists(t, workdir, "exports/inventory/team-a/keep.json", true)
		assertDiskExists(t, workdir, "exports/inventory/team-b/other.json", true)
	}
}

func TestRemoveBillyOrphans_KindDepthFollowsTemplate(t *testing.T) {
	t.Parallel()

	const tmpl = "clusters/{cluster}/{group}/{kind}/{sourceName}{extension}"
	fs := memfs.New()
	mustWriteBillyFile(t, fs, "clusters/prod/apps/Deployment/api.yaml", "a")
	mustWriteBillyFile(t, fs, "clusters/prod/apps/Service/web.yaml", "b")
	mustWriteBillyFile(t, fs, "exports/inventory/team-a/keep.json", "{}")
	mustWriteBillyFile(t, fs, "exports/inventory/team-b/other.json", "{}")

	written := []string{
		"clusters/prod/apps/Deployment/api.yaml",
		"exports/inventory/team-a/keep.json",
	}
	if err := removeBillyOrphans(fs, written, tmpl); err != nil {
		t.Fatal(err)
	}

	assertBillyExists(t, fs, "clusters/prod/apps/Deployment/api.yaml", true)
	assertBillyExists(t, fs, "clusters/prod/apps/Service/web.yaml", false)
	assertBillyExists(t, fs, "exports/inventory/team-a/keep.json", true)
	assertBillyExists(t, fs, "exports/inventory/team-b/other.json", true)
}

func TestRemoveDiskOrphans_KindDepthFollowsTemplate(t *testing.T) {
	t.Parallel()

	const tmpl = "clusters/{cluster}/{group}/{kind}/{sourceName}{extension}"
	workdir := t.TempDir()
	mustWriteDiskFile(t, workdir, "clusters/prod/apps/Deployment/api.yaml", "a")
	mustWriteDiskFile(t, workdir, "clusters/prod/apps/Service/web.yaml", "b")
	mustWriteDiskFile(t, workdir, "exports/inventory/team-a/keep.json", "{}")
	mustWriteDiskFile(t, workdir, "exports/inventory/team-b/other.json", "{}")

	written := []string{
		"clusters/prod/apps/Deployment/api.yaml",
		"exports/inventory/team-a/keep.json",
	}
	if err := removeDiskOrphans(workdir, written, tmpl); err != nil {
		t.Fatal(err)
	}

	assertDiskExists(t, workdir, "clusters/prod/apps/Deployment/api.yaml", true)
	assertDiskExists(t, workdir, "clusters/prod/apps/Service/web.yaml", false)
	assertDiskExists(t, workdir, "exports/inventory/team-a/keep.json", true)
	assertDiskExists(t, workdir, "exports/inventory/team-b/other.json", true)
}

func TestRemoveBillyOrphans_DeeperThanKindDirectoryIsNotExpanded(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/extra/keep.yaml", "a")
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/extra/stale.yaml", "b")
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/other/stale.yaml", "c")
	mustWriteBillyFile(t, fs, "prod/team-a/Service/web.yaml", "d")

	err := removeBillyOrphans(
		fs,
		[]string{"prod/team-a/Deployment/extra/keep.yaml"},
		kollectdevv1alpha1.DefaultLayoutPathTemplate,
	)
	if err != nil {
		t.Fatal(err)
	}

	assertBillyExists(t, fs, "prod/team-a/Deployment/extra/keep.yaml", true)
	assertBillyExists(t, fs, "prod/team-a/Deployment/extra/stale.yaml", false)
	assertBillyExists(t, fs, "prod/team-a/Deployment/other/stale.yaml", true)
	assertBillyExists(t, fs, "prod/team-a/Service/web.yaml", true)
}

func TestRemoveBillyOrphans_RootKindDoesNotExpand(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "Deployment/api.yaml", "a")
	mustWriteBillyFile(t, fs, "Deployment/old.yaml", "b")
	mustWriteBillyFile(t, fs, "Service/web.yaml", "c")
	mustWriteBillyFile(t, fs, "docs/guide.md", "d")

	err := removeBillyOrphans(fs, []string{"Deployment/api.yaml"}, "{kind}/{sourceName}{extension}")
	if err != nil {
		t.Fatal(err)
	}

	assertBillyExists(t, fs, "Deployment/api.yaml", true)
	assertBillyExists(t, fs, "Deployment/old.yaml", false)
	assertBillyExists(t, fs, "Service/web.yaml", true)
	assertBillyExists(t, fs, "docs/guide.md", true)
}

func TestPruneBillyOrphans_ForwardsPathTemplate(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	mustWriteBillyFile(t, fs, "prod/team-a/Deployment/api.yaml", "a")
	mustWriteBillyFile(t, fs, "prod/team-a/Service/web.yaml", "b")

	cfg := Config{Prune: true, PathTemplate: kollectdevv1alpha1.DefaultLayoutPathTemplate}
	if err := pruneBillyOrphans(fs, cfg, []string{"prod/team-a/Service/web.yaml"}); err != nil {
		t.Fatal(err)
	}

	assertBillyExists(t, fs, "prod/team-a/Deployment/api.yaml", false)
	assertBillyExists(t, fs, "prod/team-a/Service/web.yaml", true)
}
