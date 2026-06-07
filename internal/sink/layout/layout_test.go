// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func sampleItems() []collect.Item {
	return []collect.Item{
		{
			TargetNamespace: "team-a", TargetName: "deployments", Namespace: "team-a", Name: "api",
			Group: "apps", Version: "v1", Kind: "Deployment", UID: "uid-api",
			Attributes: map[string]any{"image": "nginx:1.27"},
		},
		{
			TargetNamespace: "team-a", TargetName: "deployments", Namespace: "team-a", Name: "web",
			Group: "apps", Version: "v1", Kind: "Deployment", UID: "uid-web",
			Attributes: map[string]any{"image": "nginx:1.27"},
		},
	}
}

func gitSpec(layout *kollectdevv1alpha1.LayoutSpec, format, cluster string) kollectdevv1alpha1.KollectSinkSpec {
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeGit, Cluster: cluster, Layout: layout}
	if format != "" {
		spec.Serialization = &kollectdevv1alpha1.SerializationSpec{Format: format}
	}

	return spec
}

func TestExtensionForFormat(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"yaml": ".yaml", "json": ".json", "ndjson": ".ndjson", "": ".json", "JSON": ".json"}
	for format, want := range cases {
		if got := ExtensionForFormat(format); got != want {
			t.Errorf("ExtensionForFormat(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestResolve_GitDefaultsYAMLDocument(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{Spec: gitSpec(nil, "", ""), InventoryNamespace: "team-a", InventoryName: "api"})
	if r.Mode != kollectdevv1alpha1.LayoutModeDocument {
		t.Errorf("mode = %q, want document", r.Mode)
	}
	if r.Format != kollectdevv1alpha1.SerializationFormatYAML {
		t.Errorf("format = %q, want yaml", r.Format)
	}
	if r.Extension != ".yaml" {
		t.Errorf("extension = %q", r.Extension)
	}
	if got := r.DocumentPath(); got != "inventory/team-a/api.yaml" {
		t.Errorf("DocumentPath = %q", got)
	}
	if r.Prune {
		t.Error("document mode must not auto-prune")
	}
}

func TestResolve_AutoUpgradeOnResourceExport(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(nil, "", "prod-west"),
		InventoryNamespace: "team-a", InventoryName: "api",
		ResourceExportMode: true,
	})
	if r.Mode != kollectdevv1alpha1.LayoutModePerResource {
		t.Errorf("mode = %q, want perResource (auto)", r.Mode)
	}
	if r.Content != kollectdevv1alpha1.LayoutContentManifest {
		t.Errorf("content = %q, want manifest (auto)", r.Content)
	}
	if !r.Prune {
		t.Error("perResource must auto-prune")
	}
}

func TestResolve_ExplicitDocumentOptsOutOfAutoUpgrade(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(&kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
		ResourceExportMode: true,
	})
	if r.Mode != kollectdevv1alpha1.LayoutModeDocument {
		t.Errorf("explicit document must opt out of auto-upgrade, got %q", r.Mode)
	}
}

func TestProject_DocumentYAML(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{Spec: gitSpec(nil, "", ""), InventoryNamespace: "team-a", InventoryName: "api"})
	files, err := Project(sampleItems()[:1], r)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	if files[0].Path != "inventory/team-a/api.yaml" {
		t.Errorf("path = %q", files[0].Path)
	}
	got := string(files[0].Data)
	for _, want := range []string{"kind: Deployment", "name: api", "image: nginx:1.27"} {
		if !strings.Contains(got, want) {
			t.Errorf("yaml missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") {
		t.Errorf("yaml document should not contain JSON braces:\n%s", got)
	}
}

func TestProject_DocumentJSONIsArray(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(nil, kollectdevv1alpha1.SerializationFormatJSON, ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems(), r)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Path != "inventory/team-a/api.json" {
		t.Errorf("path = %q", files[0].Path)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(files[0].Data)), "[") {
		t.Errorf("json document should be an array:\n%s", files[0].Data)
	}
}

func TestProject_DocumentNDJSON(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(nil, kollectdevv1alpha1.SerializationFormatNDJSON, ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems(), r)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(files[0].Data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson want 2 lines, got %d:\n%s", len(lines), files[0].Data)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "{") {
			t.Errorf("ndjson line not a json object: %q", line)
		}
	}
}

func TestProject_PerResourceDefaultTree(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(&kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource}, "", "prod-west"),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems(), r)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"prod-west/team-a/deployment/api.yaml": false,
		"prod-west/team-a/deployment/web.yaml": false,
	}
	if len(files) != len(want) {
		t.Fatalf("want %d files, got %d", len(want), len(files))
	}
	for _, f := range files {
		if _, ok := want[f.Path]; !ok {
			t.Errorf("unexpected path %q", f.Path)
		}
		want[f.Path] = true
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("missing path %q", p)
		}
	}
}

func TestProject_PerResourceClusterDefaultWhenUnset(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(&kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems()[:1], r)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Path != "default/team-a/deployment/api.yaml" {
		t.Errorf("path = %q, want default cluster segment", files[0].Path)
	}
}

func TestProject_PerResourceManifestContent(t *testing.T) {
	t.Parallel()
	items := []collect.Item{{
		Namespace: "team-a", Name: "api", Version: "v1", Kind: "Deployment", UID: "u1",
		Attributes: map[string]any{
			DefaultManifestKey: map[string]any{"apiVersion": "apps/v1", "kind": "Deployment"},
		},
	}}
	r := Resolve(ResolveInput{
		Spec: gitSpec(&kollectdevv1alpha1.LayoutSpec{
			Mode:    kollectdevv1alpha1.LayoutModePerResource,
			Content: kollectdevv1alpha1.LayoutContentManifest,
		}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(items, r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(files[0].Data)
	if !strings.Contains(got, "apiVersion: apps/v1") {
		t.Errorf("manifest yaml missing apiVersion:\n%s", got)
	}
	if strings.Contains(got, "targetNamespace") {
		t.Errorf("manifest content must not wrap the Item envelope:\n%s", got)
	}
}

func TestProject_ManifestContentMissingKeyErrors(t *testing.T) {
	t.Parallel()
	items := []collect.Item{{Namespace: "team-a", Name: "api", Kind: "Deployment", Attributes: map[string]any{"image": "x"}}}
	r := Resolve(ResolveInput{
		Spec: gitSpec(&kollectdevv1alpha1.LayoutSpec{
			Mode:    kollectdevv1alpha1.LayoutModePerResource,
			Content: kollectdevv1alpha1.LayoutContentManifest,
		}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	if _, err := Project(items, r); err == nil {
		t.Fatal("expected error for missing manifest attribute")
	}
}

func TestProject_CollisionFails(t *testing.T) {
	t.Parallel()
	// Two items rendering the same path (same kind/name, different uid) under a uid-less template.
	items := []collect.Item{
		{Namespace: "team-a", Name: "api", Kind: "Deployment", UID: "u1", Attributes: map[string]any{}},
		{Namespace: "team-a", Name: "api", Kind: "Deployment", UID: "u2", Attributes: map[string]any{}},
	}
	r := Resolve(ResolveInput{
		Spec:               gitSpec(&kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	if _, err := Project(items, r); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProject_SplitWritesIndexFirst(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec:               gitSpec(&kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeSplit}, "", "prod-west"),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	if !r.IndexEnabled {
		t.Fatal("split mode must enable index by default")
	}
	files, err := Project(sampleItems(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("want index + 2 items = 3 files, got %d", len(files))
	}
	if files[0].Path != "inventory/team-a/api.yaml" {
		t.Errorf("index path = %q", files[0].Path)
	}
	idx := string(files[0].Data)
	for _, want := range []string{"itemCount: 2", "schemaVersion:", "checksum:", "paths:"} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q:\n%s", want, idx)
		}
	}
}

func TestProject_AttributesContent(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec: gitSpec(&kollectdevv1alpha1.LayoutSpec{
			Mode:    kollectdevv1alpha1.LayoutModePerResource,
			Content: kollectdevv1alpha1.LayoutContentAttributes,
		}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems()[:1], r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(files[0].Data)
	if !strings.Contains(got, "image: nginx:1.27") {
		t.Errorf("attributes content missing image:\n%s", got)
	}
	if strings.Contains(got, "uid:") {
		t.Errorf("attributes content must not include identity envelope:\n%s", got)
	}
}

func TestRenderItemPath_CustomTemplateAndPlaceholders(t *testing.T) {
	t.Parallel()
	r := Resolve(ResolveInput{
		Spec: gitSpec(&kollectdevv1alpha1.LayoutSpec{
			Mode:         kollectdevv1alpha1.LayoutModePerResource,
			PathTemplate: "clusters/{cluster}/{group}/{kind}/{sourceName}{extension}",
		}, "", "prod-west"),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(sampleItems()[:1], r)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Path != "clusters/prod-west/apps/deployment/api.yaml" {
		t.Errorf("path = %q", files[0].Path)
	}
}

func TestRenderItemPath_CoreGroupOmitted(t *testing.T) {
	t.Parallel()
	items := []collect.Item{{Namespace: "team-a", Name: "cfg", Kind: "ConfigMap", UID: "u1", Attributes: map[string]any{}}}
	r := Resolve(ResolveInput{
		Spec: gitSpec(&kollectdevv1alpha1.LayoutSpec{
			Mode:         kollectdevv1alpha1.LayoutModePerResource,
			PathTemplate: "{group}/{kind}/{sourceName}{extension}",
		}, "", ""),
		InventoryNamespace: "team-a", InventoryName: "api",
	})
	files, err := Project(items, r)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Path != "configmap/cfg.yaml" {
		t.Errorf("empty group should collapse, got %q", files[0].Path)
	}
}
