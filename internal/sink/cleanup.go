// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/metrics"
	"github.com/platformrelay/kollect/internal/sink/gcs"
	"github.com/platformrelay/kollect/internal/sink/git"
	"github.com/platformrelay/kollect/internal/sink/gitlab"
	"github.com/platformrelay/kollect/internal/sink/layout"
	"github.com/platformrelay/kollect/internal/sink/local"
	"github.com/platformrelay/kollect/internal/sink/objectstore"
	"github.com/platformrelay/kollect/internal/sink/s3"
)

// Built-in backends that can retract their exports at inventory-deletion time.
var (
	_ ExportCleaner = (*git.Backend)(nil)
	_ ExportCleaner = (*gitlab.Backend)(nil)
	_ ExportCleaner = (*s3.Backend)(nil)
	_ ExportCleaner = (*gcs.Backend)(nil)
	_ ExportCleaner = (*local.Backend)(nil)
)

// ExportCleaner is implemented by backends that can remove the objects an inventory
// export previously wrote (K-28, captain call C-2a: inventory deletion must retract
// exported snapshots, not just report success over retained data).
//
// Contract: DeleteExport(ctx, paths) removes each path and its deterministic
// .part-NNNN-of-NNNN siblings and returns the sink-relative paths it actually
// deleted; paths that do not exist are not errors (cleanup is retried and must
// stay idempotent). The return lets callers tell "retracted something" from
// "nothing was ever there" (K-28: never report a clean tombstone over data that
// was only partially addressable).
type ExportCleaner interface {
	DeleteExport(ctx context.Context, paths []string) ([]string, error)
}

// CleanupExportRequest carries one inventory-deletion cleanup attempt against a
// resolved sink. The sink CR is expected to have been resolved by the caller
// (cleanupSinkExports), so cleanup does not re-resolve it.
type CleanupExportRequest struct {
	Ctx           context.Context
	Client        client.Client
	Registry      *Registry
	SinkNamespace string
	SinkName      string
	SinkUID       types.UID
	SinkSpec      kollectdevv1alpha1.KollectSinkSpec
	ObjectPath    string // canonical inventory/<ns>/<name>.json of the deleting inventory
	Generation    int64

	// LastExportedPaths carries the sink-relative paths the deleting
	// inventory's status recorded for the last successful export to this sink
	// (inventorySinkExports status). They are the retraction evidence: a
	// recorded path the cleanup attempt cannot address — the sink's
	// pathTemplate or serialization format changed after that export, or the
	// export was tree-shaped — announces retention instead of a silent
	// false-clean tombstone. Empty when nothing was ever exported (or recorded),
	// which cannot itself prove retention.
	LastExportedPaths []string
}

// CleanupExportOutcome classifies what happened to previously exported data.
type CleanupExportOutcome int

const (
	// CleanupCleaned: the backend's exported objects for this inventory were removed.
	CleanupCleaned CleanupExportOutcome = iota
	// CleanupPruned: a SupportsDelete backend (relational) received the empty
	// item set and pruned its rows (unchanged pre-K-28 behaviour).
	CleanupPruned
	// CleanupRetained: the backend cannot retract previously exported data
	// (event emitters; git layout trees whose per-resource files can interleave
	// with other inventories' trees). Callers must announce the retention loudly.
	CleanupRetained
)

// RunCleanupExport retracts one inventory's exported data at deletion time.
//
// Routing (K-28 / C-2a): relational (SupportsDelete) sinks keep the empty-export
// prune; backends implementing ExportCleaner have their objects deleted (git
// document mode, S3/GCS, local); everything else is reported retained so the
// controller emits a Warning Event naming the path instead of the old silent no-op.
//
// The returned outcome is meaningful only when err == nil: on error the outcome
// is a zero value (CleanupCleaned) that callers must ignore (cleanupSinkExports
// does).
func RunCleanupExport(req CleanupExportRequest) (CleanupExportOutcome, error) {
	if req.Registry == nil {
		return CleanupCleaned, kollecterrors.Terminal(fmt.Errorf("sink registry is not configured"))
	}
	if req.SinkSpec.Type == "" {
		return CleanupCleaned, kollecterrors.Terminal(fmt.Errorf("sink spec is required for cleanup of %q", req.SinkName))
	}

	backend, release, err := acquireBackend(
		req.Ctx, req.Client, req.Registry, req.SinkNamespace, req.SinkName, req.SinkUID, req.SinkSpec,
	)
	if err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("acquire backend for cleanup of %q: %w", req.SinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return CleanupCleaned, err
	}
	defer release()

	invNS, invName := objectstore.InventoryFromObjectPath(req.ObjectPath)

	if backend.Capabilities().SupportsDelete {
		envelope, merr := export.MarshalEnvelope([]collect.Item{}, export.Metadata{
			Generation: req.Generation,
			ExportedAt: time.Now().UTC(),
		})
		if merr != nil {
			return CleanupPruned, kollecterrors.Terminal(merr)
		}

		// RunExportEnvelope already classifies the error and increments
		// kollect_sink_errors_total; do not count the failure twice here. The
		// empty export rewrites the tombstone path; its written path is not
		// recorded — the object is being deleted.
		if _, rerr := RunExportEnvelope(ExportEnvelopeRequest{
			Ctx:           req.Ctx,
			Client:        req.Client,
			Registry:      req.Registry,
			SinkNamespace: req.SinkNamespace,
			SinkName:      req.SinkName,
			SinkUID:       req.SinkUID,
			ObjectPath:    req.ObjectPath,
			Envelope:      envelope,
			SinkSpec:      req.SinkSpec,
		}); rerr != nil {
			return CleanupPruned, rerr
		}

		return CleanupPruned, nil
	}

	cleaner, canDelete := backend.(ExportCleaner)
	if !canDelete {
		// Streams (Kafka/NATS) physically cannot unsent events; unknown future
		// snapshot backends stay conservative. The caller announces retention.
		return CleanupRetained, nil
	}

	paths := cleanupCandidatePaths(req.SinkSpec, invNS, invName, req.Generation)
	treeMode := isGitLayoutFamily(req.SinkSpec.Type) &&
		!layout.Resolve(layout.ResolveInput{
			Spec:               req.SinkSpec,
			InventoryNamespace: invNS,
			InventoryName:      invName,
			Generation:         req.Generation,
		}).IsDocument()

	// A {generation} path template leaves one object per past generation: git
	// document mode never prunes (layout.go Prune = mode != document) and object
	// stores have no prune at all; cleanup only addresses the current path.
	// Parquet ignores the template: its hive directory is swept whole.
	staleGenerations := strings.Contains(req.SinkSpec.PathTemplate, "{generation}") &&
		!objectstore.IsParquetFormat(req.SinkSpec)

	deleted, derr := cleaner.DeleteExport(req.Ctx, paths)
	if derr != nil {
		// Backends classify their own transport/config errors (git engines call
		// ClassifyExportError); anything unclassified stays transient so cleanup
		// retries instead of wedging the deletion.
		derr = classifyCleanupFailure(req.SinkName, derr)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(derr)).Inc()

		return CleanupCleaned, derr
	}

	// gitlab merge_request mode lands retraction on the target branch only when
	// a human merges the deletion MR: never a clean tombstone, and a later merge
	// of a stale unmerged export branch could even resurrect objects.
	mrMediated := req.SinkSpec.Type == kollectdevv1alpha1.SnapshotSinkTypeGitLab &&
		req.SinkSpec.GitLab != nil && req.SinkSpec.GitLab.MergeRequest != nil &&
		req.SinkSpec.GitLab.MergeRequest.Mode == string(gitlab.MergeRequestModeBranchMR)

	// Retention is announced only on evidence (K-28 must not train operators to
	// ignore the warning on the common fully-retracted deletion). Addressed
	// paths are those the attempt enumerated (candidates) or actually removed
	// (the deleted report: part siblings the matchers swept beyond the exact
	// candidates). A recorded exported path outside that set — the sink's
	// pathTemplate or serialization format changed after the last export, or
	// the export wrote tree-shaped per-resource files no document-side candidate
	// addresses — is real retention. With layout.mode unset an export whose
	// items all carry one embedded-object attribute auto-upgrades to the
	// per-resource tree with zero spec signal (layout_export.go
	// inferResourceLayoutHints); the recorded paths are what make that case
	// visible without announcing on every default-sink deletion.
	//
	// treeMode/staleGenerations/mrMediated stay structural: recorded paths only
	// describe the LAST export, so past-generation objects, an explicit
	// non-document layout (per-resource/split) whose files interleave with other
	// inventories' trees under shared templates, and an unmerged deletion MR are
	// announced regardless of what was recorded.
	if treeMode || staleGenerations || mrMediated ||
		recordedExportPathsUnaddressed(req.LastExportedPaths, paths, deleted) {
		return CleanupRetained, nil
	}

	return CleanupCleaned, nil
}

// recordedExportPathsUnaddressed reports whether any recorded exported path
// falls outside the paths the cleanup attempt enumerated (candidates) or
// actually removed (the backend's deleted report). Such a path is real
// retention evidence: the sink's pathTemplate or serialization format changed
// after that export, or the export wrote tree-shaped files no document-side
// candidate addresses.
func recordedExportPathsUnaddressed(recorded, candidates, deleted []string) bool {
	addressed := make(map[string]struct{}, len(candidates)+len(deleted))
	for _, p := range candidates {
		addressed[p] = struct{}{}
	}
	for _, p := range deleted {
		addressed[p] = struct{}{}
	}

	for _, p := range recorded {
		if _, ok := addressed[p]; !ok {
			return true
		}
	}

	return false
}

// cleanupCandidatePaths mirrors how RunExportEnvelope derives the export path, so
// the tombstone addresses exactly what the export wrote. For git-family sinks it
// additionally addresses the layout sidecars (split index, per-set manifest).
func cleanupCandidatePaths(
	spec kollectdevv1alpha1.KollectSinkSpec,
	invNS, invName string,
	generation int64,
) []string {
	if !isGitLayoutFamily(spec.Type) {
		return []string{objectstore.ObjectPath(spec, invNS, invName, generation)}
	}

	resolved := layout.Resolve(layout.ResolveInput{
		Spec:               spec,
		InventoryNamespace: invNS,
		InventoryName:      invName,
		Generation:         generation,
	})

	paths := []string{resolved.DocumentPath()}
	if resolved.IndexEnabled {
		paths = append(paths, resolved.IndexPath())
	}

	setResolved := resolved
	setResolved.InventoryName = baseInventoryName(resolved.InventoryName)
	paths = append(paths, setResolved.SetManifestPath())

	return dedupeStrings(paths)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

// classifyCleanupFailure mirrors classifyExportFailure: a terminal error keeps
// its class (finalizer wedge + operator action), anything else stays transient
// so deletion cleanup retries.
func classifyCleanupFailure(sinkName string, err error) error {
	if kollecterrors.IsTerminal(err) {
		return fmt.Errorf("cleanup delete for %q: %w", sinkName, err)
	}

	return kollecterrors.Transient(fmt.Errorf("cleanup delete for %q: %w", sinkName, err))
}
