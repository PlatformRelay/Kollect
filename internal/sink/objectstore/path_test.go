// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package objectstore

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestObjectPath_defaultTemplate(t *testing.T) {
	t.Parallel()

	got := ObjectPath(kollectdevv1alpha1.KollectSinkSpec{Type: "s3"}, "team-a", "deployments", 7)
	want := "inventory/team-a/deployments.json"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestObjectPath_customTemplate(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:         "s3",
		Cluster:      "prod-west",
		PathTemplate: "{cluster}/{namespace}/{name}{extension}",
	}
	got := ObjectPath(spec, "team-a", "deployments", 7)
	want := "prod-west/team-a/deployments.json"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestObjectPath_parquetLayout(t *testing.T) {
	t.Parallel()

	parquetPath := ObjectPath(kollectdevv1alpha1.KollectSinkSpec{
		Type:    "s3",
		Cluster: "prod-west",
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{
			Format: FormatParquet,
		},
	}, "team-a", "deployments", 7)
	want := "inventory/cluster=prod-west/ns=team-a/name=deployments/generation=7.parquet"
	if parquetPath != want {
		t.Fatalf("parquet path = %q, want %q", parquetPath, want)
	}
}

func TestRenderPathTemplate_generation(t *testing.T) {
	t.Parallel()

	got := RenderPathTemplate("snapshots/{namespace}/{name}-{generation}{extension}", PathVars{
		Namespace:  "team-a",
		Name:       "deployments",
		Generation: 42,
		Extension:  ".json",
	})
	want := "snapshots/team-a/deployments-42.json"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestValidatePathTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		tpl     string
		wantErr bool
	}{
		{name: "empty ok", tpl: ""},
		{name: "default ok", tpl: DefaultPathTemplate},
		{name: "custom ok", tpl: "{cluster}/{namespace}/{name}.json"},
		{name: "missing namespace", tpl: "{cluster}/{name}.json", wantErr: true},
		{name: "absolute", tpl: "/inventory/{namespace}/{name}.json", wantErr: true},
		{name: "traversal", tpl: "inventory/../{namespace}/{name}.json", wantErr: true},
		{name: "unknown placeholder", tpl: "{namespace}/{name}/{foo}.json", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePathTemplate(tc.tpl)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidatePathTemplate(%q) = %v, wantErr %v", tc.tpl, err, tc.wantErr)
			}
		})
	}
}

func TestInventoryFromObjectPath(t *testing.T) {
	t.Parallel()

	ns, name := InventoryFromObjectPath("inventory/team-a/deployments.json")
	if ns != "team-a" || name != "deployments" {
		t.Fatalf("got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("prod-west/team-a/deployments.json")
	if ns != "team-a" || name != "deployments" {
		t.Fatalf("custom layout got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("inventory/cluster=prod/ns=team-a/name=deployments/generation=3.parquet")
	if ns != "team-a" || name != "deployments" {
		t.Fatalf("parquet got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("inventory/cluster/rollup.json")
	if ns != "cluster" || name != "rollup" {
		t.Fatalf("cluster rollup got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("")
	if ns != "" || name != "" {
		t.Fatalf("empty path got (%q, %q)", ns, name)
	}
}

func TestParquetObjectPathAndIsParquetFormat(t *testing.T) {
	t.Parallel()

	got := ParquetObjectPath(" prod-west ", "team-a", "deployments", 9)
	want := "inventory/cluster=prod-west/ns=team-a/name=deployments/generation=9.parquet"
	if got != want {
		t.Fatalf("parquet path = %q, want %q", got, want)
	}

	if !IsParquetFormat(kollectdevv1alpha1.KollectSinkSpec{
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{Format: FormatParquet},
	}) {
		t.Fatal("expected parquet format detection")
	}
	if IsParquetFormat(kollectdevv1alpha1.KollectSinkSpec{}) {
		t.Fatal("expected non-parquet default")
	}
}

func TestRenderPathTemplateDefaults(t *testing.T) {
	t.Parallel()

	got := RenderPathTemplate("", PathVars{Namespace: "team-a", Name: "inv"})
	want := "inventory/team-a/inv.json"
	if got != want {
		t.Fatalf("default template = %q, want %q", got, want)
	}

	got = RenderPathTemplate("{name}{extension}", PathVars{Name: "snap", Extension: "json"})
	if got != "snap.json" {
		t.Fatalf("extension normalization = %q", got)
	}
}
