// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
)

type fakeBackend struct {
	exportPayload []byte
	exportPath    string
	exportCalled  bool

	files          []git.FileEntry
	prune          bool
	pruneKeepPaths []string
	pruneOwner     string
	filesCalled    bool
}

func (f *fakeBackend) Type() string                   { return "fake" }
func (f *fakeBackend) Capabilities() cap.Capabilities { return cap.SnapshotStore() }

func (f *fakeBackend) Export(_ context.Context, payload []byte, path string) error {
	f.exportCalled = true
	f.exportPayload = payload
	f.exportPath = path

	return nil
}

type fakeTreeBackend struct {
	fakeBackend
}

func (f *fakeTreeBackend) ExportFiles(_ context.Context, files []git.FileEntry, opts git.ExportFilesOptions) error {
	f.filesCalled = true
	f.files = files
	f.prune = opts.Prune && !opts.SuppressPrune
	f.pruneKeepPaths = opts.PruneKeepPaths
	f.pruneOwner = opts.PruneOwner

	return nil
}

func testEnvelope(t *testing.T) []byte {
	t.Helper()

	items := []collect.Item{
		{Namespace: "team-a", Name: "api", Version: "v1", Kind: "Deployment", UID: "u1", Attributes: map[string]any{"image": "nginx"}},
		{Namespace: "team-a", Name: "web", Version: "v1", Kind: "Deployment", UID: "u2", Attributes: map[string]any{"image": "nginx"}},
	}
	env, err := export.MarshalEnvelope(items, export.Metadata{ExportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}

	return env
}

func testResourceEnvelope(t *testing.T, manifestKey string) []byte {
	t.Helper()

	manifest := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"namespace": "team-a"},
		"spec":       map[string]any{"replicas": 1},
	}

	items := []collect.Item{
		{
			Namespace: "team-a", Name: "api", Version: "v1", Kind: "Deployment", UID: "u1",
			Attributes: map[string]any{
				manifestKey: manifest,
				"image":     "nginx",
			},
		},
		{
			Namespace: "team-a", Name: "web", Version: "v1", Kind: "Deployment", UID: "u2",
			Attributes: map[string]any{
				manifestKey: manifest,
				"image":     "nginx",
			},
		},
	}

	env, err := export.MarshalEnvelope(items, export.Metadata{ExportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}

	return env
}

func TestResolveSnapshotExport_NonGitUsesDefaultPath(t *testing.T) {
	t.Parallel()
	be := &fakeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeS3}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.exportCalled || be.filesCalled {
		t.Fatalf("non-git should use Export, got exportCalled=%v filesCalled=%v", be.exportCalled, be.filesCalled)
	}
	if be.exportPath != "inventory/team-a/api.json" {
		t.Errorf("path = %q", be.exportPath)
	}
}

func TestResolveSnapshotExport_GitJSONDocumentKeepsEnvelope(t *testing.T) {
	t.Parallel()
	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:          kollectdevv1alpha1.SinkTypeGit,
		Serialization: &kollectdevv1alpha1.SerializationSpec{Format: kollectdevv1alpha1.SerializationFormatJSON},
	}
	env := testEnvelope(t)

	plan, err := resolveSnapshotExport(be, spec, env, "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.exportCalled || be.filesCalled {
		t.Fatalf("git+json document must use Export (legacy envelope), exportCalled=%v filesCalled=%v", be.exportCalled, be.filesCalled)
	}
	if plan.objectPath != "inventory/team-a/api.json" {
		t.Errorf("objectPath = %q", plan.objectPath)
	}
	if string(be.exportPayload) != string(env) {
		t.Error("git+json document must write the canonical envelope unchanged")
	}
}

func TestResolveSnapshotExport_GitDefaultYAMLDocumentTree(t *testing.T) {
	t.Parallel()
	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.filesCalled || be.exportCalled {
		t.Fatalf("git yaml document must use ExportFiles, filesCalled=%v exportCalled=%v", be.filesCalled, be.exportCalled)
	}
	if len(be.files) != 1 || be.files[0].Path != "inventory/team-a/api.yaml" {
		t.Fatalf("files = %+v", be.files)
	}
	if be.prune {
		t.Error("document mode must not prune")
	}
	if !strings.Contains(string(be.files[0].Data), "kind: Deployment") {
		t.Errorf("yaml content unexpected:\n%s", be.files[0].Data)
	}
}

// TestResolveSnapshotExport_GitDefaultYAMLDropsCompletenessMarker pins the REL-02
// scope boundary: the human-readable YAML layout projection (ADR-0419) serializes
// BARE items and carries no ExportEnvelope metadata, so the multipart completeness
// marker (partIndex/partTotal + generation) — and every other envelope header — is
// absent from a default Git/GitLab (YAML) sink's payload. Payload-level torn-set
// detection therefore does NOT apply to YAML sinks; tree-mode multipart sets instead
// carry a per-set *.manifest.json sidecar (shipped). If the payload ever gains
// envelope headers, update ADR-0405's scope wording alongside it.
func TestResolveSnapshotExport_GitDefaultYAMLDropsCompletenessMarker(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{Namespace: "team-a", Name: "api", Version: "v1", Kind: "Deployment", UID: "u1"},
		{Namespace: "team-a", Name: "web", Version: "v1", Kind: "Deployment", UID: "u2"},
	}
	// A marked multipart part: partIndex=1 of 3, generation 7.
	marked, err := export.MarshalEnvelope(items, export.Metadata{
		Generation: 7,
		PartIndex:  1,
		PartTotal:  3,
		ExportedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: the JSON envelope handed in really does carry the marker.
	if m := export.EnvelopeMetaFromPayload(marked); m.PartIndex != 1 || m.PartTotal != 3 || m.Generation != 7 {
		t.Fatalf("input envelope lost its marker before projection: %+v", m)
	}

	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit} // default format = YAML

	plan, err := resolveSnapshotExport(be, spec, marked, "team-a", "api", 7, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.filesCalled || len(be.files) != 1 {
		t.Fatalf("git yaml document must project one file via ExportFiles, filesCalled=%v files=%d", be.filesCalled, len(be.files))
	}

	// The YAML projection is bare items — none of the envelope headers survive.
	yaml := string(be.files[0].Data)
	for _, field := range []string{"partIndex", "partTotal", "generation", "schemaVersion", "checksum", "itemCount"} {
		if strings.Contains(yaml, field) {
			t.Fatalf("YAML layout projection must NOT carry envelope field %q (torn-set detection does not apply to YAML sinks), got:\n%s",
				field, yaml)
		}
	}
	// It is still the real inventory content, just marker-less.
	if !strings.Contains(yaml, "name: api") || !strings.Contains(yaml, "name: web") {
		t.Fatalf("expected bare item YAML, got:\n%s", yaml)
	}
}

func TestResolveSnapshotExport_GitPerResourceTree(t *testing.T) {
	t.Parallel()
	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    kollectdevv1alpha1.SinkTypeGit,
		Cluster: "prod-west",
		Layout:  &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.filesCalled || !be.prune {
		t.Fatalf("perResource must use ExportFiles with prune, filesCalled=%v prune=%v", be.filesCalled, be.prune)
	}
	if len(be.files) != 2 {
		t.Fatalf("want 2 files, got %d", len(be.files))
	}
	for _, f := range be.files {
		if !strings.HasPrefix(f.Path, "prod-west/team-a/deployment/") {
			t.Errorf("unexpected path %q", f.Path)
		}
	}
}

func TestResolveSnapshotExport_GitAutoInfersResourceModeFromEnvelope(t *testing.T) {
	t.Parallel()
	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit}

	plan, err := resolveSnapshotExport(be, spec, testResourceEnvelope(t, "payload"), "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.filesCalled || !be.prune {
		t.Fatalf("resource-mode envelope must use ExportFiles with prune, filesCalled=%v prune=%v", be.filesCalled, be.prune)
	}
	if len(be.files) != 2 {
		t.Fatalf("want 2 files, got %d", len(be.files))
	}

	for _, f := range be.files {
		if !strings.HasPrefix(f.Path, "default/team-a/deployment/") {
			t.Fatalf("unexpected per-resource path %q", f.Path)
		}
		data := string(f.Data)
		if !strings.Contains(data, "apiVersion: apps/v1") || !strings.Contains(data, "kind: Deployment") {
			t.Fatalf("expected manifest yaml, got:\n%s", data)
		}
		if strings.Contains(data, "attributes:") {
			t.Fatalf("manifest content must not include item envelope:\n%s", data)
		}
	}
}

func TestResolveSnapshotExport_GitYAMLDocumentFallbackWithoutFileExporter(t *testing.T) {
	t.Parallel()
	// A git-type backend that does NOT implement FileExporter falls back to single-document Export.
	be := &fakeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json", teamAAPI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !be.exportCalled {
		t.Fatal("fallback should call Export")
	}
	if plan.objectPath != "inventory/team-a/api.yaml" {
		t.Errorf("objectPath = %q", plan.objectPath)
	}
}

var teamAAPI = InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "api"}

// TestResolveSnapshotExportOwnershipStableAcrossParts: the owner comes from the identity, so
// generations and multipart suffixes never create a new owner, while kind, namespace, name and sink
// cluster each do.
func TestResolveSnapshotExportOwnershipStableAcrossParts(t *testing.T) {
	t.Parallel()
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "git", Layout: &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource}}
	owner := func(id InventoryIdentity, pathName, cluster string, generation int64, index, total int) string {
		t.Helper()
		localSpec := spec
		localSpec.Cluster = cluster
		be := &fakeTreeBackend{}
		items, err := export.ItemsFromPayload(testEnvelope(t))
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := export.MarshalEnvelope(items, export.Metadata{PartIndex: index, PartTotal: total})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := resolveSnapshotExport(be, localSpec, envelope, "team-a", pathName, generation, "inventory/team-a/api.json", id, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.run(t.Context()); err != nil {
			t.Fatal(err)
		}
		return be.pruneOwner
	}
	for _, name := range []string{"api", "api.part-0002-of-0002"} {
		id := InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: name}
		want, err := json.Marshal([5]string{"v2", InventoryKindNamespaced, "default", "team-a", name})
		if err != nil {
			t.Fatal(err)
		}
		for _, part := range []struct {
			name         string
			index, total int
			generation   int64
		}{
			{name, 0, 0, 1}, {name, 1, 1, 2}, {name + ".part-0001-of-0002", 1, 2, 3}, {name + ".part-0002-of-0002", 2, 2, 4},
		} {
			if got := owner(id, part.name, "", part.generation, part.index, part.total); got != string(want) {
				t.Errorf("name=%s part=%d/%d: owner=%q want=%q", part.name, part.index, part.total, got, want)
			}
		}
	}
	base := owner(teamAAPI, "api", "", 1, 0, 0)
	for _, other := range []string{
		owner(InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-b", Name: "api"}, "api", "", 1, 0, 0),
		owner(InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "other"}, "api", "", 1, 0, 0),
		owner(teamAAPI, "api", "other", 1, 0, 0),
		owner(InventoryIdentity{Kind: InventoryKindCluster, Name: "api"}, "api", "", 1, 0, 0),
	} {
		if other == base {
			t.Fatal("distinct inventories share ownership")
		}
	}
}

func TestEmptyGitTreePolicyLeavesDocumentAndOtherSinksUnchanged(t *testing.T) {
	t.Parallel()
	tree := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "git"}
	if exportsEmptyGitTree(tree, spec) {
		t.Fatal("implicit document layout changed empty policy")
	}
	spec.Layout = &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument}
	if exportsEmptyGitTree(tree, spec) {
		t.Fatal("explicit document layout changed empty policy")
	}
	spec.Layout.Mode = kollectdevv1alpha1.LayoutModePerResource
	if !exportsEmptyGitTree(tree, spec) {
		t.Fatal("explicit resource tree must reconcile empty ownership")
	}
	if exportsEmptyGitTree(&fakeBackend{}, spec) {
		t.Fatal("backend without file export opted into tree deletion")
	}
	spec.Type = "s3"
	if exportsEmptyGitTree(tree, spec) {
		t.Fatal("non-git snapshot changed empty policy")
	}
}

func TestCleanupCandidatePathsPreservesSuffixShapedInventoryName(t *testing.T) {
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "git"}
	for _, p := range cleanupCandidatePaths(spec, "team-a", "api.part-0002-of-0002", 1) {
		if !strings.Contains(p, "api.part-0002-of-0002") {
			t.Fatalf("cleanup addressed another inventory: %q", p)
		}
	}
}
