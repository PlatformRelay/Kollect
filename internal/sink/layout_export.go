// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/layout"
)

// FileExporter is implemented by git/gitlab backends that can write a projected layout tree in a
// single commit (ADR-0419). Backends that do not implement it fall back to single-document export.
type FileExporter interface {
	ExportFiles(ctx context.Context, files []git.FileEntry, prune bool) error
}

// snapshotExport bundles the export closure with the representative object path used for commit
// context, fingerprinting, and metrics.
type snapshotExport struct {
	objectPath string
	run        func(ctx context.Context) error
}

func isGitLayoutFamily(sinkType string) bool {
	return sinkType == kollectdevv1alpha1.SnapshotSinkTypeGit ||
		sinkType == kollectdevv1alpha1.SnapshotSinkTypeGitLab
}

// resolveSnapshotExport decides how to project and write a snapshot for a sink (ADR-0419).
//
// Git/GitLab sinks serialize to the resolved format (yaml by default) and, for non-document layouts,
// write a per-resource tree via FileExporter. The legacy json+document case writes the canonical
// envelope unchanged so serialization.format: json pins pre-ADR-0419 behaviour. All other sinks keep
// the existing single-payload export.
func resolveSnapshotExport(
	backend Backend,
	spec kollectdevv1alpha1.KollectSinkSpec,
	envelope []byte,
	invNS, invName string,
	generation int64,
	defaultObjectPath string,
) (snapshotExport, error) {
	if !isGitLayoutFamily(spec.Type) {
		return snapshotExport{
			objectPath: defaultObjectPath,
			run:        func(ctx context.Context) error { return backend.Export(ctx, envelope, defaultObjectPath) },
		}, nil
	}

	resolved := layout.Resolve(layout.ResolveInput{
		Spec:               spec,
		InventoryNamespace: invNS,
		InventoryName:      invName,
		Generation:         generation,
	})

	fileExporter, canTree := backend.(FileExporter)

	// serialization.format: json + document mode pins pre-ADR-0419 behaviour: write the canonical
	// JSON envelope unchanged so existing JSON consumers keep working.
	if resolved.IsDocument() && resolved.Format == kollectdevv1alpha1.SerializationFormatJSON {
		docPath := resolved.DocumentPath()

		return snapshotExport{
			objectPath: docPath,
			run:        func(ctx context.Context) error { return backend.Export(ctx, envelope, docPath) },
		}, nil
	}

	if meta := export.EnvelopeMetaFromPayload(envelope); !meta.ExportedAt.IsZero() {
		resolved.ExportedAt = meta.ExportedAt.UTC().Format(time.RFC3339)
	}

	items, err := export.ItemsFromPayload(envelope)
	if err != nil {
		return snapshotExport{}, err
	}

	files, err := layout.Project(items, resolved)
	if err != nil {
		return snapshotExport{}, err
	}

	// Non-tree backends (test stubs / unusual backends) cannot write a per-resource tree. Document
	// mode projects exactly one file, so we can still export it as a single payload with the resolved
	// format (e.g. a YAML Items list). Multi-file layouts require FileExporter; fall back to the
	// canonical envelope rather than dropping resources.
	if !canTree {
		if len(files) == 1 {
			f := files[0]

			return snapshotExport{
				objectPath: f.Path,
				run:        func(ctx context.Context) error { return backend.Export(ctx, f.Data, f.Path) },
			}, nil
		}

		docPath := resolved.DocumentPath()

		return snapshotExport{
			objectPath: docPath,
			run:        func(ctx context.Context) error { return backend.Export(ctx, envelope, docPath) },
		}, nil
	}

	gitFiles := make([]git.FileEntry, 0, len(files))
	for _, f := range files {
		gitFiles = append(gitFiles, git.FileEntry{Path: f.Path, Data: f.Data})
	}

	prune := resolved.Prune

	return snapshotExport{
		objectPath: resolved.DocumentPath(),
		run:        func(ctx context.Context) error { return fileExporter.ExportFiles(ctx, gitFiles, prune) },
	}, nil
}
