// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
)

var (
	clusterPlatform    = InventoryIdentity{Kind: InventoryKindCluster, Name: "platform"}
	namespacedPlatform = InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "cluster", Name: "platform"}
)

const platformObjectPath = "inventory/cluster/platform.json"

// identityRemote is a bare Git remote driven through the real Git backend (CLI engine, file
// remote), so these tests cover RunExportEnvelope -> layout -> owner -> ownership engine -> commit.
type identityRemote struct {
	t      *testing.T
	work   string
	remote string
	spec   kollectdevv1alpha1.KollectSinkSpec
}

func newIdentityRemote(t *testing.T) *identityRemote {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git CLI required for the owned-prune export tests: %v", err)
	}
	DisableBackendPoolForTest()
	t.Cleanup(func() {
		EnableBackendPoolForTest()
		ResetBackendPoolForTest()
		ResetBreakersForTest()
	})
	work := t.TempDir()
	remote := filepath.Join(work, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "--initial-branch=main", remote).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("init bare: %s: %v", out, err)
	}

	return &identityRemote{t: t, work: work, remote: remote, spec: kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.SinkTypeGit,
		Endpoint: "file://" + remote,
		Layout:   &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}}
}

// identityItem projects to default/<namespace>/<kind>/<name>.yaml and carries the exporting
// inventory's label, so an overwrite of another inventory's file is visible in the bytes.
func identityItem(namespace, kind, name, writer string) collect.Item {
	return collect.Item{
		Namespace: namespace, Name: name, Kind: kind, Version: "v1", UID: "uid-" + kind + "-" + name,
		Attributes: map[string]any{"payload": map[string]any{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]any{
				"namespace": namespace, "name": name,
				"labels": map[string]any{"kollect.dev/test-writer": writer},
			},
		}},
	}
}

func (r *identityRemote) export(id InventoryIdentity, objectPath string, items []collect.Item, index, total int, plan *PrunePlan) error {
	r.t.Helper()
	ResetBreakersForTest()
	meta := export.Metadata{Generation: 1, ExportedAt: time.Now().UTC()}
	if total > 1 {
		meta.PartIndex, meta.PartTotal = index, total
	}
	envelope, err := export.MarshalEnvelope(items, meta)
	if err != nil {
		r.t.Fatalf("marshal envelope: %v", err)
	}
	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           r.t.Context(),
		Registry:      NewRegistry(),
		SinkNamespace: "default",
		SinkName:      "identity-git",
		ObjectPath:    export.PartitionObjectPath(objectPath, index, total),
		Inventory:     id,
		Envelope:      envelope,
		SinkSpec:      r.spec,
		PrunePlan:     plan,
	})

	return err
}

// files clones main and maps every file outside .git to its content. An empty remote has none.
func (r *identityRemote) files() map[string]string {
	r.t.Helper()
	out := map[string]string{}
	heads, err := exec.Command("git", "--git-dir", r.remote, "branch", "--list", "main").CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		r.t.Fatalf("list branches: %s: %v", heads, err)
	}
	if strings.TrimSpace(string(heads)) == "" {
		return out
	}
	clone, err := os.MkdirTemp(r.work, "clone-")
	if err != nil {
		r.t.Fatal(err)
	}
	_ = os.RemoveAll(clone)
	if out, err := exec.Command("git", "clone", "--quiet", "--branch", "main", "file://"+r.remote, clone).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		r.t.Fatalf("clone: %s: %v", out, err)
	}
	walkErr := filepath.WalkDir(clone, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(p) //nolint:gosec // G304: test fixture
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(clone, p)
		out[filepath.ToSlash(rel)] = string(data)

		return nil
	})
	if walkErr != nil {
		r.t.Fatal(walkErr)
	}

	return out
}

func dataPaths(files map[string]string) []string {
	var out []string
	for p := range files {
		if !strings.HasPrefix(p, ".kollect-prune/") {
			out = append(out, p)
		}
	}
	sort.Strings(out)

	return out
}

func requirePaths(t *testing.T, files map[string]string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := dataPaths(files); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("repository files = %v, want %v", got, want)
	}
}

// TestRunExportEnvelope_clusterInventoryPrunesOwnFiles (IEI-3): a KollectClusterInventory's tree
// export deletes its own stale files again, down to an empty snapshot.
func TestRunExportEnvelope_clusterInventoryPrunesOwnFiles(t *testing.T) {
	r := newIdentityRemote(t)
	api := identityItem("team-a", "Deployment", "api", "L")
	web := identityItem("team-a", "Deployment", "web", "L")

	for _, step := range []struct {
		items []collect.Item
		want  []string
	}{
		{[]collect.Item{api, web}, []string{"default/team-a/deployment/api.yaml", "default/team-a/deployment/web.yaml"}},
		{[]collect.Item{api}, []string{"default/team-a/deployment/api.yaml"}},
		{nil, nil},
	} {
		if err := r.export(clusterPlatform, platformObjectPath, step.items, 1, 1, nil); err != nil {
			t.Fatalf("cluster inventory export: %v", err)
		}
		requirePaths(t, r.files(), step.want...)
	}
}

// TestRunExportEnvelope_clusterKindAndClusterNamespaceIsolated (IEI-2): the two inventories that
// share object path inventory/cluster/platform.json own disjoint trees and never delete each
// other's files, including on an empty snapshot.
func TestRunExportEnvelope_clusterKindAndClusterNamespaceIsolated(t *testing.T) {
	r := newIdentityRemote(t)
	clusterRes := "default/clusterrole/admin.yaml"
	nsRes := "default/cluster/configmap/settings.yaml"

	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{identityItem("", "ClusterRole", "admin", "L")}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.export(namespacedPlatform, platformObjectPath, []collect.Item{identityItem("cluster", "ConfigMap", "settings", "N")}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	requirePaths(t, r.files(), clusterRes, nsRes)

	if err := r.export(clusterPlatform, platformObjectPath, nil, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	files := r.files()
	if _, ok := files[clusterRes]; ok {
		t.Fatalf("cluster inventory file %s still present after its empty snapshot", clusterRes)
	}
	requirePaths(t, files, nsRes)

	if err := r.export(namespacedPlatform, platformObjectPath, nil, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	requirePaths(t, r.files())
}

// TestRunExportEnvelope_sharedPathRejectedNamesOwner (IEI-2): when both inventories project the
// same file, the second export is rejected, names the owning inventory and its record, and
// leaves the owner's bytes alone.
func TestRunExportEnvelope_sharedPathRejectedNamesOwner(t *testing.T) {
	r := newIdentityRemote(t)
	shared := "default/team-a/deployment/api.yaml"
	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{identityItem("team-a", "Deployment", "api", "L")}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	before := r.files()

	err := r.export(namespacedPlatform, platformObjectPath, []collect.Item{identityItem("team-a", "Deployment", "api", "N")}, 1, 1, nil)
	owner, _ := git.InventoryPruneOwner(InventoryKindCluster, "default", "", "platform")
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("namespaced export of the cluster inventory's path: err = %v, want a terminal rejection", err)
	}
	for _, want := range []string{"belongs to another inventory", `KollectClusterInventory platform (cluster "default")`, fmt.Sprintf(".kollect-prune/%x.json", sha256.Sum256([]byte(owner)))} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("rejection %q does not name %q", err, want)
		}
	}
	after := r.files()
	if after[shared] != before[shared] || !strings.Contains(after[shared], "kollect.dev/test-writer: L") {
		t.Fatalf("rejected export changed the owner's bytes:\n%s", after[shared])
	}
}

// TestRunExportEnvelope_multipartForeignManifestRejectedOnPart1: a set whose deterministic set
// manifest belongs to another inventory is rejected on part 1, before any part is committed.
func TestRunExportEnvelope_multipartForeignManifestRejectedOnPart1(t *testing.T) {
	r := newIdentityRemote(t)
	const manifest = "inventory/cluster/platform.manifest.json"

	plan := NewPrunePlan()
	for i, item := range []collect.Item{identityItem("cluster", "ConfigMap", "one", "N"), identityItem("cluster", "ConfigMap", "two", "N")} {
		if err := r.export(namespacedPlatform, platformObjectPath, []collect.Item{item}, i+1, 2, plan); err != nil {
			t.Fatalf("namespaced set part %d: %v", i+1, err)
		}
	}
	before := r.files()
	if _, ok := before[manifest]; !ok {
		t.Fatalf("namespaced set wrote no manifest; files %v", dataPaths(before))
	}

	err := r.export(clusterPlatform, platformObjectPath, []collect.Item{identityItem("", "ClusterRole", "admin", "L")}, 1, 2, NewPrunePlan())
	if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), "belongs to another inventory") || !strings.Contains(err.Error(), manifest) {
		t.Fatalf("part 1 of a set whose manifest is foreign: err = %v, want a terminal rejection naming %s", err, manifest)
	}
	if after := r.files(); strings.Join(dataPaths(after), ",") != strings.Join(dataPaths(before), ",") {
		t.Fatalf("rejected part 1 committed files: before %v after %v", dataPaths(before), dataPaths(after))
	}
}

// recordingTreeBackend records ExportFiles calls without writing anything.
type recordingTreeBackend struct {
	mu    sync.Mutex
	calls []git.ExportFilesOptions
	kinds []string // commit-context kind of each call
}

func (b *recordingTreeBackend) Type() string { return "git" }

func (b *recordingTreeBackend) Capabilities() Capabilities { return cap.SnapshotStore() }

func (b *recordingTreeBackend) Export(context.Context, []byte, string) error { return nil }

func (b *recordingTreeBackend) ExportFiles(ctx context.Context, _ []git.FileEntry, opts git.ExportFilesOptions) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, opts)
	commitCtx, _ := git.CommitContextFromContext(ctx)
	b.kinds = append(b.kinds, commitCtx.Kind)

	return nil
}

func recordingRegistry(b *recordingTreeBackend) *Registry {
	reg := NewRegistry()
	reg.Register("git", func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) { return b, nil })

	return reg
}

func runRecorded(t *testing.T, b *recordingTreeBackend, id InventoryIdentity, objectPath string) error {
	t.Helper()
	ResetBreakersForTest()
	envelope, err := export.MarshalEnvelope([]collect.Item{identityItem("team-a", "Deployment", "api", "x")}, export.Metadata{Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx: t.Context(), Registry: recordingRegistry(b), SinkNamespace: "default", SinkName: "recorded",
		ObjectPath: objectPath, Inventory: id, Envelope: envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypeGit, Endpoint: "https://example.com/inventory.git",
			Layout: &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
		},
	})

	return err
}

// TestRunExportEnvelope_ownerFromIdentity (IEI-1): the owner handed to the ownership engine is the
// kind-qualified owner of the request's identity, with prune requested for both kinds.
func TestRunExportEnvelope_ownerFromIdentity(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() { EnableBackendPoolForTest(); ResetBackendPoolForTest(); ResetBreakersForTest() })
	for _, tc := range []struct {
		id         InventoryIdentity
		objectPath string
		want       string
	}{
		{clusterPlatform, platformObjectPath, `["v2","KollectClusterInventory","default","","platform"]`},
		{namespacedPlatform, platformObjectPath, `["v2","KollectInventory","default","cluster","platform"]`},
		{InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "apps"}, "inventory/team-a/apps.json", `["v2","KollectInventory","default","team-a","apps"]`},
	} {
		b := &recordingTreeBackend{}
		if err := runRecorded(t, b, tc.id, tc.objectPath); err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if len(b.calls) != 1 || b.calls[0].PruneOwner != tc.want || b.calls[0].SuppressPrune || !b.calls[0].Prune {
			t.Fatalf("%s: ExportFiles options = %+v, want owner %s with prune requested", tc.id, b.calls, tc.want)
		}
	}
}

// TestRunExportEnvelope_commitContextCarriesKind (IEI-11): the backend's commit context names the
// inventory kind, which the GitLab sink needs to give the two same-path kinds separate branches.
func TestRunExportEnvelope_commitContextCarriesKind(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() { EnableBackendPoolForTest(); ResetBackendPoolForTest(); ResetBreakersForTest() })
	for _, id := range []InventoryIdentity{clusterPlatform, namespacedPlatform} {
		b := &recordingTreeBackend{}
		if err := runRecorded(t, b, id, platformObjectPath); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(b.kinds) != 1 || b.kinds[0] != id.Kind {
			t.Fatalf("%s: commit-context kinds = %v, want [%s]", id, b.kinds, id.Kind)
		}
	}
}

// TestRunExportEnvelope_identityPathMismatchRejected: an identity that disagrees with the object
// path is a terminal error before the backend is reached.
func TestRunExportEnvelope_identityPathMismatchRejected(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() { EnableBackendPoolForTest(); ResetBackendPoolForTest(); ResetBreakersForTest() })
	for name, tc := range map[string]struct {
		id         InventoryIdentity
		objectPath string
	}{
		"namespace":                  {InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-b", Name: "apps"}, "inventory/team-a/apps.json"},
		"name":                       {InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "web"}, "inventory/team-a/apps.json"},
		"cluster kind, namespace":    {InventoryIdentity{Kind: InventoryKindCluster, Name: "apps"}, "inventory/team-a/apps.json"},
		"cluster kind with ns":       {InventoryIdentity{Kind: InventoryKindCluster, Namespace: "cluster", Name: "platform"}, platformObjectPath},
		"namespaced kind without ns": {InventoryIdentity{Kind: InventoryKindNamespaced, Name: "platform"}, platformObjectPath},
		"unknown kind":               {InventoryIdentity{Kind: "KollectSomething", Namespace: "cluster", Name: "platform"}, platformObjectPath},
		"suffix is not the base name": {
			InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "apps"}, "inventory/team-a/apps.part-0001-of-0002.json",
		},
	} {
		b := &recordingTreeBackend{}
		err := runRecorded(t, b, tc.id, tc.objectPath)
		if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), "inventory identity") {
			t.Fatalf("%s: err = %v, want a terminal identity error", name, err)
		}
		if len(b.calls) != 0 {
			t.Fatalf("%s: backend reached despite the identity/path mismatch: %+v", name, b.calls)
		}
	}
}

// TestRunExportEnvelope_treeExportRequiresIdentity: a tree export without an identity is terminal.
func TestRunExportEnvelope_treeExportRequiresIdentity(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() { EnableBackendPoolForTest(); ResetBackendPoolForTest(); ResetBreakersForTest() })
	b := &recordingTreeBackend{}
	err := runRecorded(t, b, InventoryIdentity{}, "inventory/team-a/apps.json")
	if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), "requires an inventory identity") {
		t.Fatalf("identity-less tree export: err = %v, want terminal identity requirement", err)
	}
	if len(b.calls) != 0 {
		t.Fatalf("backend reached without an identity: %+v", b.calls)
	}
}
