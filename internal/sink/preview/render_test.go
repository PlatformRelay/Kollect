// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package preview

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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

func TestRender_layoutSamplePathsCapped(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type:   kollectdevv1alpha1.SnapshotSinkTypeGit,
		Layout: &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}, "git-backup")
	if out.Layout == nil {
		t.Fatal("expected layout preview")
	}
	if got := len(out.Layout.SamplePaths); got != 3 {
		t.Fatalf("sample paths capped at 3, got %d: %v", got, out.Layout.SamplePaths)
	}
}

func TestRender_layoutProjectErrorFallsBackToDocumentPath(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
		Layout: &kollectdevv1alpha1.LayoutSpec{
			Mode:         kollectdevv1alpha1.LayoutModePerResource,
			PathTemplate: "collide/{kind}{extension}",
		},
	}, "git-backup")
	if out.Layout == nil {
		t.Fatal("expected layout preview")
	}
	want := "inventory/team-a/api.yaml"
	if len(out.Layout.SamplePaths) != 1 || out.Layout.SamplePaths[0] != want {
		t.Fatalf("project error should fall back to document path, got %v want [%q]", out.Layout.SamplePaths, want)
	}
}

func TestRender_existingProvisioningWarning(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type:         kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			Table:  "inventory_items",
			Schema: "public",
		},
	}, "warehouse")
	if out.ProvisioningMode != kollectdevv1alpha1.ProvisioningModeExisting {
		t.Fatalf("provisioning mode = %q, want existing", out.ProvisioningMode)
	}
	if !containsWarning(out.Warnings, "provisioning.mode=existing") {
		t.Fatalf("expected existing-mode warning, got %v", out.Warnings)
	}
}

func TestRender_s3ObjectPath(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeS3}, "bucket")
	if out.ObjectPath != "inventory/team-a/api.json" {
		t.Fatalf("object path = %q, want inventory/team-a/api.json", out.ObjectPath)
	}
}

func TestRender_mongodbPreview(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeMongoDB,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			Database:   "inventory",
			Collection: "items",
		},
	}, "mongo")
	if out.MongoDB == nil || len(out.MongoDB.ExpectedIndexKeys) == 0 {
		t.Fatalf("expected mongodb preview, got %#v", out.MongoDB)
	}
	if !containsWarning(out.Warnings, "mongodb: documents upserted into inventory.items") {
		t.Fatalf("expected mongodb upsert warning, got %v", out.Warnings)
	}
}

func TestRender_kafkaPreview(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type:  kollectdevv1alpha1.EventSinkTypeKafka,
		Kafka: &kollectdevv1alpha1.KafkaSpec{Topic: "inventory.events"},
	}, "bus")
	if out.Kafka == nil || out.Kafka.Topic != "inventory.events" {
		t.Fatalf("expected kafka topic preview, got %#v", out.Kafka)
	}
}

func TestRender_parquetSerializationWarning(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type:          kollectdevv1alpha1.SnapshotSinkTypeS3,
		Serialization: &kollectdevv1alpha1.SerializationSpec{Format: kollectdevv1alpha1.SerializationFormatParquet},
	}, "lake")
	if out.SerializationFormat != kollectdevv1alpha1.SerializationFormatParquet {
		t.Fatalf("format = %q, want parquet", out.SerializationFormat)
	}
	if !containsWarning(out.Warnings, "serialization.format=parquet") {
		t.Fatalf("expected parquet warning, got %v", out.Warnings)
	}
}

func containsWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
