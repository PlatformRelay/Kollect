// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"time"

	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/objectstore"
)

type commitContextKey struct{}

// WithCommitContext attaches export attribution fields for git commit rendering (ADR-0415).
func WithCommitContext(ctx context.Context, cc CommitContext) context.Context {
	return context.WithValue(ctx, commitContextKey{}, cc)
}

// CommitContextFromContext returns commit context attached by the export pipeline, if any.
func CommitContextFromContext(ctx context.Context) (CommitContext, bool) {
	cc, ok := ctx.Value(commitContextKey{}).(CommitContext)

	return cc, ok
}

// CommitContextFromExport builds commit context from envelope metadata and inventory path.
func CommitContextFromExport(
	envelope []byte,
	objectPath string,
	sinkCluster string,
	sinkName string,
) CommitContext {
	invNS, invName := objectstore.InventoryFromObjectPath(objectPath)
	meta := export.EnvelopeMetaFromPayload(envelope)

	cluster := meta.Cluster
	if cluster == "" {
		cluster = sinkCluster
	}
	if cluster == "" {
		cluster = "default"
	}

	exportedAt := meta.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}

	return CommitContext{
		Namespace:  invNS,
		Name:       invName,
		Cluster:    cluster,
		Generation: meta.Generation,
		ExportGen:  meta.Generation,
		ItemCount:  meta.ItemCount,
		Checksum:   meta.Checksum,
		ExportedAt: exportedAt,
		Path:       objectPath,
		SinkName:   sinkName,
	}
}
