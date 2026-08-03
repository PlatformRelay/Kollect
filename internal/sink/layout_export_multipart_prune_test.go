//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
)

// deploymentItem builds a per-resource manifest item that projects to
// default/team-a/deployment/<name>.yaml under the auto-inferred git layout.
func deploymentItem(name string) collect.Item {
	manifest := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"namespace": "team-a", "name": name},
	}

	return collect.Item{
		Namespace: "team-a", Name: name, Kind: "Deployment", Version: "v1", UID: "uid-" + name,
		Attributes: map[string]any{"payload": manifest},
	}
}

func newBareRemote(t *testing.T) (work, remote string) {
	t.Helper()
	work = t.TempDir()
	remote = filepath.Join(work, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("init bare: %s: %v", out, err)
	}

	return work, remote
}

func cloneMain(t *testing.T, work, remote string) string {
	t.Helper()
	clone, err := os.MkdirTemp(work, "clone-")
	if err != nil {
		t.Fatalf("mkdir clone: %v", err)
	}
	// git clone requires the target to be empty/absent.
	if err := os.RemoveAll(clone); err != nil {
		t.Fatalf("reset clone dir: %v", err)
	}
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", "file://"+remote, clone).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone: %s: %v", out, err)
	}

	return clone
}

func gitLayoutSpec(remote string, prune bool) kollectdevv1alpha1.KollectSinkSpec {
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.SinkTypeGit,
		Endpoint: "file://" + remote,
	}
	if prune {
		spec.Git = &kollectdevv1alpha1.GitSpec{Prune: true}
	}

	return spec
}

func assertResourceFile(t *testing.T, clone, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(clone, rel)) //nolint:gosec // G304: test fixture
	if err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
	}
	if !strings.Contains(string(data), "kind: Deployment") {
		t.Fatalf("%s missing manifest yaml:\n%s", rel, data)
	}
}

func assertResourceAbsent(t *testing.T, clone, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(clone, rel)); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be pruned, stat err=%v", rel, err)
	}
}

// exportPart marshals one part envelope and drives it through the sink exactly as the controller
// loop does: partitioned object path + optional shared PrunePlan accumulator.
func exportPart(
	t *testing.T,
	spec kollectdevv1alpha1.KollectSinkSpec,
	items []collect.Item,
	generation int64,
	index, total int,
	plan *PrunePlan,
) error {
	t.Helper()
	meta := export.Metadata{Generation: generation, ExportedAt: time.Now().UTC()}
	if total > 1 {
		meta.PartIndex = index
		meta.PartTotal = total
	}
	envelope, err := export.MarshalEnvelope(items, meta)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      NewRegistry(),
		SinkNamespace: "default",
		SinkName:      "resource-git",
		ObjectPath:    export.PartitionObjectPath("inventory/team-a/apps.json", index, total),
		Envelope:      envelope,
		SinkSpec:      spec,
		PrunePlan:     plan,
	})
}

func withGitCLI(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	DisableBackendPoolForTest()
	t.Cleanup(func() {
		EnableBackendPoolForTest()
		ResetBackendPoolForTest()
	})
}

// Test 1 (reproduction): two disjoint parts written per-part with NO shared PrunePlan.
// Before the fix each part prunes directory-scoped against only its own file, so part 2's prune
// deletes part 1's api.yaml (last-part-wins silent data loss). Both files must survive.
func TestRunExportEnvelope_MultipartGit_PerPartPruneDropsPriorParts(t *testing.T) {
	withGitCLI(t)
	work, remote := newBareRemote(t)

	spec := gitLayoutSpec(remote, false)
	parts := [][]collect.Item{{deploymentItem("api")}, {deploymentItem("web")}}
	for i, items := range parts {
		if err := exportPart(t, spec, items, 1, i+1, len(parts), nil); err != nil {
			t.Fatalf("export part %d: %v", i+1, err)
		}
	}

	clone := cloneMain(t, work, remote)
	assertResourceFile(t, clone, "default/team-a/deployment/api.yaml")
	assertResourceFile(t, clone, "default/team-a/deployment/web.yaml")
}

// Test 2 (invariant): a shared PrunePlan must prune EXACTLY ONCE against the union of all parts.
// Gen1 writes {api,web}. Gen2 is two parts {api},{db} sharing one PrunePlan: api survives (present
// in the union), db is written, and web -- a resource dropped since the prior generation -- is
// pruned. This distinguishes the correct union fix from merely disabling prune.
func TestRunExportEnvelope_MultipartGit_UnionPrunePreservesPartsDropsStale(t *testing.T) {
	withGitCLI(t)
	work, remote := newBareRemote(t)

	spec := gitLayoutSpec(remote, false)

	// Gen1: single (non-part) export of both resources.
	if err := exportPart(t, spec, []collect.Item{deploymentItem("api"), deploymentItem("web")}, 1, 1, 1, nil); err != nil {
		t.Fatalf("gen1 export: %v", err)
	}
	gen1 := cloneMain(t, work, remote)
	assertResourceFile(t, gen1, "default/team-a/deployment/api.yaml")
	assertResourceFile(t, gen1, "default/team-a/deployment/web.yaml")

	// Gen2: two parts sharing one accumulator.
	plan := NewPrunePlan()
	parts := [][]collect.Item{{deploymentItem("api")}, {deploymentItem("db")}}
	for i, items := range parts {
		if err := exportPart(t, spec, items, 2, i+1, len(parts), plan); err != nil {
			t.Fatalf("gen2 export part %d: %v", i+1, err)
		}
	}

	gen2 := cloneMain(t, work, remote)
	assertResourceFile(t, gen2, "default/team-a/deployment/api.yaml")   // regression: survives the union
	assertResourceFile(t, gen2, "default/team-a/deployment/db.yaml")    // new resource written
	assertResourceAbsent(t, gen2, "default/team-a/deployment/web.yaml") // stale: pruned once, at the end
}

// Test 3 (never-per-part under spec prune): with spec.git.prune=true and THREE parts, a non-final
// part must NOT prune. Without suppression, part 2's per-part prune (keep = part 2's file) would
// delete part 1's api.yaml because the OR with the spec-level prune flag keeps prune on. All three
// same-generation resources must survive; prune runs once, on the final part, against the union.
func TestRunExportEnvelope_MultipartGit_SpecPruneNeverRunsPerPart(t *testing.T) {
	withGitCLI(t)
	work, remote := newBareRemote(t)

	spec := gitLayoutSpec(remote, true) // spec.git.prune=true -- would defeat opts.Prune=false alone

	plan := NewPrunePlan()
	parts := [][]collect.Item{
		{deploymentItem("api")},
		{deploymentItem("web")},
		{deploymentItem("db")},
	}
	for i, items := range parts {
		if err := exportPart(t, spec, items, 1, i+1, len(parts), plan); err != nil {
			t.Fatalf("export part %d: %v", i+1, err)
		}
	}

	clone := cloneMain(t, work, remote)
	assertResourceFile(t, clone, "default/team-a/deployment/api.yaml") // part 1 -- must survive part 2's prune
	assertResourceFile(t, clone, "default/team-a/deployment/web.yaml") // part 2 -- must survive part 3's prune
	assertResourceFile(t, clone, "default/team-a/deployment/db.yaml")  // part 3 (final)
}
