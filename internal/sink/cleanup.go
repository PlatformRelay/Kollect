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

// Git-family backends release the deleting inventory's ownership record (ADR-0422).
var (
	_ OwnershipReleaser = (*git.Backend)(nil)
	_ OwnershipReleaser = (*gitlab.Backend)(nil)
)

// OwnershipReleaser is implemented by the Git-family backends, whose tree exports keep per-inventory
// ownership records (ADR-0422). ReleaseExport removes opts.Owner's record and, unless opts.KeepFiles,
// the candidate paths and every path that record lists, never a path another owner's record lists.
// An absent record is not an error. It returns the paths it removed, the record included.
type OwnershipReleaser interface {
	ReleaseExport(ctx context.Context, paths []string, opts git.ReleaseOptions) ([]string, error)
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

	// Inventory names the deleting inventory. Git-family sinks derive the owner whose ownership
	// record the deletion releases from it (ADR-0422); it must match ObjectPath.
	Inventory InventoryIdentity

	// LastExportedPaths carries the sink-relative paths the deleting
	// inventory's status recorded for the last successful export to this sink
	// (inventorySinkExports status). They are the retraction evidence: a
	// recorded path the cleanup attempt cannot address — the sink's
	// pathTemplate or serialization format changed after that export, or the
	// export was tree-shaped — announces retention instead of a silent
	// false-clean tombstone. Empty when nothing was ever exported (or recorded).
	LastExportedPaths []string

	// ExportPathsUnrecorded marks a sink the inventory exported to before
	// lastExportPaths was recorded (status shows a past export, no paths). With
	// no evidence of what that export wrote, a retraction cannot prove it
	// addressed everything, so the outcome announces retention.
	ExportPathsUnrecorded bool

	// SharedExportIdentity marks an inventory whose export identity is also
	// rendered by another live inventory: a KollectClusterInventory X exports
	// as inventory/cluster/X, exactly like a KollectInventory X in namespace
	// "cluster". Retracting would delete the other inventory's export, so the
	// retraction is skipped and announced.
	SharedExportIdentity bool
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
	// CleanupRetainedByPolicy: the snapshot sink's deletionPolicy is Retain
	// (the default, ADR-0421); exported objects were deliberately left in place.
	// Object stores were not contacted; a git/gitlab sink released only the
	// inventory's ownership record (ADR-0422).
	CleanupRetainedByPolicy
	// CleanupRetainedSharedIdentity: the sink's deletionPolicy is Delete, but
	// another live inventory renders the same export identity, so retracting
	// would delete its export. Nothing was deleted; callers announce it.
	CleanupRetainedSharedIdentity
)

// retractableSnapshotTypes are the sink types whose backends implement
// ExportCleaner. For the object stores the deletionPolicy is decided before the
// backend is built, so their Retain deletion never needs working credentials;
// git and gitlab are always contacted to release the ownership record.
var retractableSnapshotTypes = map[string]struct{}{
	kollectdevv1alpha1.SnapshotSinkTypeGit:    {},
	kollectdevv1alpha1.SnapshotSinkTypeGitLab: {},
	kollectdevv1alpha1.SnapshotSinkTypeS3:     {},
	kollectdevv1alpha1.SnapshotSinkTypeGCS:    {},
	local.TypeName:                            {},
}

// RunCleanupExport retracts one inventory's exported data at deletion time.
//
// Routing (K-28 / C-2a, ADR-0421): relational (SupportsDelete) sinks keep the
// empty-export prune; backends implementing ExportCleaner (S3/GCS, local) act
// on the sink's deletionPolicy — Retain (the default) reports
// CleanupRetainedByPolicy without building the backend, Delete retracts the
// inventory's objects unless the export identity is shared with another live
// inventory; everything else is reported retained so the controller emits a
// Warning Event naming the path instead of a silent no-op.
//
// Git and GitLab sinks always reach the backend (ADR-0422): the deleting
// inventory's ownership record is released under every policy. Retain and a
// shared identity release only the record and keep every file; Delete also
// retracts the candidates and the inventory's recorded files, never a path
// another inventory's record lists. A failed release is returned like a failed
// retraction, so the finalizer keeps retrying.
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

	gitFamily := isGitLayoutFamily(req.SinkSpec.Type)
	owner := ""
	if gitFamily {
		var err error
		if owner, err = cleanupPruneOwner(req); err != nil {
			return CleanupCleaned, kollecterrors.Terminal(err)
		}
	} else if _, retractable := retractableSnapshotTypes[req.SinkSpec.Type]; retractable {
		if outcome, decided := retractionPrecheck(req); decided {
			return outcome, nil
		}
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
		return CleanupPruned, pruneRelationalExport(req)
	}

	if gitFamily {
		return releaseGitExport(req, backend, owner, invNS, invName)
	}

	cleaner, canDelete := backend.(ExportCleaner)
	if !canDelete {
		// Streams (Kafka/NATS) physically cannot unsent events; unknown future
		// snapshot backends stay conservative. The caller announces retention.
		return CleanupRetained, nil
	}

	if outcome, decided := retractionPrecheck(req); decided {
		return outcome, nil
	}

	paths := cleanupCandidatePaths(req.SinkSpec, invNS, invName, req.Generation)

	// A {generation} path template leaves one object per past generation: git
	// document mode never prunes (layout.go Prune = mode != document) and object
	// stores have no prune at all; cleanup only addresses the current path.
	// Parquet ignores the template: its hive directory is swept whole.
	staleGenerations := strings.Contains(req.SinkSpec.PathTemplate, "{generation}") &&
		!objectstore.IsParquetFormat(req.SinkSpec)

	deleted, derr := cleaner.DeleteExport(req.Ctx, paths)
	if derr != nil {
		return CleanupCleaned, cleanupFailure(req.SinkName, derr)
	}

	return retractionOutcome(req, paths, deleted, staleGenerations, false), nil
}

// releaseGitExport is the Git-family cleanup (ADR-0422): one ReleaseExport call
// that removes the deleting inventory's ownership record and, when the policy
// retracts, its candidates and recorded files.
func releaseGitExport(req CleanupExportRequest, backend Backend, owner, invNS, invName string) (CleanupExportOutcome, error) {
	releaser, ok := backend.(OwnershipReleaser)
	if !ok {
		return CleanupCleaned, kollecterrors.Terminal(fmt.Errorf(
			"sink %q: %s backend cannot release ownership records", req.SinkName, req.SinkSpec.Type))
	}

	kept, keepFiles := retractionPrecheck(req)
	paths := cleanupCandidatePaths(req.SinkSpec, invNS, invName, req.Generation)
	resolved := layout.Resolve(layout.ResolveInput{
		Spec:               req.SinkSpec,
		InventoryNamespace: invNS,
		InventoryName:      invName,
		Generation:         req.Generation,
	})

	// The commit context names the deleting inventory for the commit subject and, in GitLab
	// branchMR mode, the feature branch, so the backends do not re-derive it from a path.
	commitCtx := git.CommitContextFromObjectPath(resolved.DocumentPath(), resolved.Cluster)
	commitCtx.Kind, commitCtx.Namespace, commitCtx.Name = req.Inventory.Kind, invNS, invName
	ctx := git.WithCommitContext(req.Ctx, commitCtx)

	deleted, derr := releaser.ReleaseExport(ctx, paths, git.ReleaseOptions{Owner: owner, KeepFiles: keepFiles})
	if derr != nil {
		return CleanupCleaned, cleanupFailure(req.SinkName, derr)
	}
	if keepFiles {
		return kept, nil
	}

	treeMode := !resolved.IsDocument()
	staleGenerations := strings.Contains(req.SinkSpec.PathTemplate, "{generation}")

	return retractionOutcome(req, paths, deleted, staleGenerations, treeMode), nil
}

// cleanupPruneOwner derives the deleting inventory's prune owner exactly as the
// export does (inventoryPruneOwner on the layout resolved from the sink spec),
// after checking the identity against the object path.
func cleanupPruneOwner(req CleanupExportRequest) (string, error) {
	if req.Inventory.IsZero() {
		return "", fmt.Errorf("git cleanup of %s requires an inventory identity (kind, namespace, name)", req.ObjectPath)
	}
	invNS, invName := objectstore.InventoryFromObjectPath(req.ObjectPath)
	if err := req.Inventory.checkObjectPath(req.ObjectPath, invNS, invName, 0, 0); err != nil {
		return "", err
	}
	resolved := layout.Resolve(layout.ResolveInput{
		Spec:               req.SinkSpec,
		InventoryNamespace: invNS,
		InventoryName:      invName,
		Generation:         req.Generation,
	})

	return inventoryPruneOwner(resolved, req.Inventory)
}

// cleanupFailure classifies a failed retraction or release and counts it.
// Backends classify their own transport/config errors (git engines call
// ClassifyExportError); anything unclassified stays transient so cleanup
// retries instead of wedging the deletion.
func cleanupFailure(sinkName string, err error) error {
	err = classifyCleanupFailure(sinkName, err)
	metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

	return err
}

// retractionOutcome decides whether a completed retraction is a clean
// tombstone or announced retention.
func retractionOutcome(
	req CleanupExportRequest,
	paths, deleted []string,
	staleGenerations, treeMode bool,
) CleanupExportOutcome {
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
	//
	// ExportPathsUnrecorded is the absence of that evidence: the inventory
	// exported before paths were recorded, so nothing proves the candidates
	// covered what it wrote.
	if treeMode || staleGenerations || mrMediated || req.ExportPathsUnrecorded ||
		recordedExportPathsUnaddressed(req.LastExportedPaths, paths, deleted) {
		return CleanupRetained
	}

	return CleanupCleaned
}

// pruneRelationalExport is the relational (SupportsDelete) cleanup: the empty
// item set is exported so the backend prunes the inventory's rows (unchanged
// pre-K-28 behaviour; deletionPolicy does not apply).
func pruneRelationalExport(req CleanupExportRequest) error {
	envelope, merr := export.MarshalEnvelope([]collect.Item{}, export.Metadata{
		Generation: req.Generation,
		ExportedAt: time.Now().UTC(),
	})
	if merr != nil {
		return kollecterrors.Terminal(merr)
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
		return rerr
	}

	return nil
}

// RetractsOnDeletion reports whether inventory deletion retracts objects through
// a sink with this spec: an ExportCleaner snapshot type whose deletionPolicy is
// Delete (ADR-0421). Callers use it to skip work that only a retraction needs.
func RetractsOnDeletion(spec kollectdevv1alpha1.KollectSinkSpec) bool {
	_, retractable := retractableSnapshotTypes[spec.Type]

	return retractable && kollectdevv1alpha1.EffectiveDeletionPolicy(&spec) == kollectdevv1alpha1.DeletionPolicyDelete
}

// retractionPrecheck applies the decisions that need no backend contact before
// an ExportCleaner retracts anything: the sink's deletionPolicy (Retain unless
// Delete, ADR-0421) and the shared-identity guard. decided is false when the
// retraction may proceed.
func retractionPrecheck(req CleanupExportRequest) (CleanupExportOutcome, bool) {
	if kollectdevv1alpha1.EffectiveDeletionPolicy(&req.SinkSpec) != kollectdevv1alpha1.DeletionPolicyDelete {
		return CleanupRetainedByPolicy, true
	}

	if req.SharedExportIdentity {
		return CleanupRetainedSharedIdentity, true
	}

	return CleanupCleaned, false
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

	// Cleanup receives the inventory identity, not a multipart object name.
	paths = append(paths, resolved.SetManifestPath())

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
