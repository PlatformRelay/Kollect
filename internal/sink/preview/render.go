// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package preview renders read-only sink implications without side effects (ADR-0416).
package preview

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/layout"
	"github.com/konih/kollect/internal/sink/objectstore"
	"github.com/konih/kollect/internal/sink/postgres"
)

const syntheticSampleSource = "synthetic"

// Render builds status.preview for a normalized sink spec (ADR-0416).
func Render(spec kollectdevv1alpha1.KollectSinkSpec, sinkName string) *kollectdevv1alpha1.SinkPreviewStatus {
	now := metav1.NewTime(time.Now().UTC())
	mode := kollectdevv1alpha1.EffectiveProvisioningMode(&spec)
	format := kollectdevv1alpha1.EffectiveSerializationFormat(&spec)

	preview := &kollectdevv1alpha1.SinkPreviewStatus{
		RenderedAt:          &now,
		InputSampleSource:   syntheticSampleSource,
		ProvisioningMode:    mode,
		SerializationFormat: format,
	}

	var warnings []string
	if mode == kollectdevv1alpha1.ProvisioningModeExisting {
		warnings = append(warnings, "provisioning.mode=existing: kollect will NOT create destination resources; preflight verifies existence")
	}

	switch spec.Type {
	case kollectdevv1alpha1.SnapshotSinkTypeGit, kollectdevv1alpha1.SnapshotSinkTypeGitLab:
		resolved := layout.Resolve(layout.ResolveInput{
			Spec:               spec,
			InventoryNamespace: "team-a",
			InventoryName:      "api",
			Generation:         1,
		})
		path := resolved.DocumentPath()
		preview.ObjectPath = path
		preview.Layout = renderLayoutPreview(resolved)
		ctx := git.CommitContext{
			Namespace:  "team-a",
			Name:       "api",
			Cluster:    defaultCluster(spec.Cluster),
			Generation: 1,
			ExportGen:  1,
			ItemCount:  42,
			Checksum:   "sha256:a1b2c3d4e5f6",
			ExportedAt: time.Now().UTC(),
			Path:       path,
			SinkName:   sinkName,
		}
		subject, body := git.RenderCommitPreview(spec, ctx)
		preview.Git = &kollectdevv1alpha1.GitPreviewStatus{
			SampleCommitSubject: subject,
			SampleCommitBody:    body,
		}
	case kollectdevv1alpha1.SnapshotSinkTypeS3, kollectdevv1alpha1.SnapshotSinkTypeGCS, kollectdevv1alpha1.SnapshotSinkTypeAzureBlob:
		preview.ObjectPath = objectstore.ObjectPath(spec, "team-a", "api", 1)
	case kollectdevv1alpha1.DatabaseSinkTypePostgres:
		if spec.Postgres != nil {
			preview.Postgres = &kollectdevv1alpha1.PostgresPreviewStatus{
				ExpectedDDL: postgres.ExpectedCreateTableDDL(spec.Postgres.Schema, spec.Postgres.Table),
			}
		}
	case kollectdevv1alpha1.DatabaseSinkTypeMongoDB:
		if spec.MongoDB != nil {
			preview.MongoDB = &kollectdevv1alpha1.MongoDBPreviewStatus{
				ExpectedIndexKeys: []string{
					"inventory_namespace", "inventory_name", "target_name", "source_uid",
				},
			}
			warnings = append(warnings, fmt.Sprintf("mongodb: documents upserted into %s.%s",
				spec.MongoDB.Database, spec.MongoDB.Collection))
		}
	case kollectdevv1alpha1.EventSinkTypeKafka:
		if spec.Kafka != nil {
			preview.Kafka = &kollectdevv1alpha1.KafkaPreviewStatus{Topic: spec.Kafka.Topic}
		}
	}

	if format == kollectdevv1alpha1.SerializationFormatParquet {
		warnings = append(warnings, "serialization.format=parquet: typed identity columns + JSON attributes + optional hotAttributes")
	}

	preview.Warnings = warnings
	return preview
}

func renderLayoutPreview(r layout.ResolvedLayout) *kollectdevv1alpha1.LayoutPreviewStatus {
	p := &kollectdevv1alpha1.LayoutPreviewStatus{
		Mode:    r.Mode,
		Content: r.Content,
		Prune:   r.Prune,
	}

	if r.IsDocument() {
		p.SamplePaths = []string{r.DocumentPath()}

		return p
	}

	files, err := layout.Project(samplePreviewItems(), r)
	if err != nil {
		p.SamplePaths = []string{r.DocumentPath()}

		return p
	}

	const maxSamplePaths = 3
	paths := make([]string, 0, maxSamplePaths)
	for i, f := range files {
		if i >= maxSamplePaths {
			break
		}
		paths = append(paths, f.Path)
	}
	p.SamplePaths = paths

	return p
}

func samplePreviewItems() []collect.Item {
	manifest := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"spec":       map[string]any{"replicas": 3},
	}

	return []collect.Item{
		{
			TargetNamespace: "team-a", TargetName: "api", Namespace: "team-a", Name: "api",
			Group: "apps", Version: "v1", Kind: "Deployment", UID: "uid-api",
			Attributes: map[string]any{layout.DefaultManifestKey: manifest, "image": "nginx:1.27"},
		},
		{
			TargetNamespace: "team-a", TargetName: "web", Namespace: "team-a", Name: "web",
			Group: "apps", Version: "v1", Kind: "Deployment", UID: "uid-web",
			Attributes: map[string]any{layout.DefaultManifestKey: manifest, "image": "nginx:1.27"},
		},
	}
}

func defaultCluster(cluster string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return "default"
	}

	return cluster
}
