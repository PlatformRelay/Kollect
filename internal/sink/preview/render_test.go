// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package preview

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestRender_postgresDDL(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			Table:  "inventory_items",
			Schema: "public",
		},
	}, "warehouse")
	if out.Postgres == nil || !strings.Contains(out.Postgres.ExpectedDDL, "CREATE TABLE IF NOT EXISTS") {
		t.Fatalf("expected postgres DDL preview, got %#v", out.Postgres)
	}
}

func TestRender_gitCommitSubject(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit}, "git-backup")
	if out.Git == nil || out.Git.SampleCommitSubject == "" {
		t.Fatalf("expected git preview, got %#v", out.Git)
	}
}

func TestRender_gitDefaultLayoutPreview(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit}, "git-backup")
	if out.SerializationFormat != kollectdevv1alpha1.SerializationFormatYAML {
		t.Errorf("git default format = %q, want yaml", out.SerializationFormat)
	}
	if out.Layout == nil || out.Layout.Mode != kollectdevv1alpha1.LayoutModeDocument {
		t.Fatalf("expected document layout preview, got %#v", out.Layout)
	}
	if want := "inventory/team-a/api.yaml"; len(out.Layout.SamplePaths) != 1 || out.Layout.SamplePaths[0] != want {
		t.Errorf("sample paths = %v, want [%q]", out.Layout.SamplePaths, want)
	}
}

func TestRender_gitPerResourceLayoutPreview(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type:    kollectdevv1alpha1.SnapshotSinkTypeGit,
		Cluster: "prod-west",
		Layout:  &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}, "git-backup")
	if out.Layout == nil || out.Layout.Mode != kollectdevv1alpha1.LayoutModePerResource || !out.Layout.Prune {
		t.Fatalf("expected pruning perResource preview, got %#v", out.Layout)
	}
	if len(out.Layout.SamplePaths) == 0 {
		t.Fatal("expected sample paths")
	}
	for _, p := range out.Layout.SamplePaths {
		if !strings.HasPrefix(p, "prod-west/team-a/deployment/") {
			t.Errorf("unexpected sample path %q", p)
		}
	}
}
