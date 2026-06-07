// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/git"
)

type fakeBackend struct {
	exportPayload []byte
	exportPath    string
	exportCalled  bool

	files       []git.FileEntry
	prune       bool
	filesCalled bool
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

func (f *fakeTreeBackend) ExportFiles(_ context.Context, files []git.FileEntry, prune bool) error {
	f.filesCalled = true
	f.files = files
	f.prune = prune

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

func TestResolveSnapshotExport_NonGitUsesDefaultPath(t *testing.T) {
	t.Parallel()
	be := &fakeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeS3}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json")
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

	plan, err := resolveSnapshotExport(be, spec, env, "team-a", "api", 1, "inventory/team-a/api.json")
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

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json")
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

func TestResolveSnapshotExport_GitPerResourceTree(t *testing.T) {
	t.Parallel()
	be := &fakeTreeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    kollectdevv1alpha1.SinkTypeGit,
		Cluster: "prod-west",
		Layout:  &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json")
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

func TestResolveSnapshotExport_GitYAMLDocumentFallbackWithoutFileExporter(t *testing.T) {
	t.Parallel()
	// A git-type backend that does NOT implement FileExporter falls back to single-document Export.
	be := &fakeBackend{}
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit}

	plan, err := resolveSnapshotExport(be, spec, testEnvelope(t), "team-a", "api", 1, "inventory/team-a/api.json")
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
