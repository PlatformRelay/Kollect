// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/git"
	"github.com/platformrelay/kollect/internal/sink/layout"
)

// PrunePlan accumulates the projected file paths of every part in a multipart git-layout export.
// A single plan is shared across all parts of one export so that, for a prune-bearing layout, prune
// runs EXACTLY ONCE -- against the union of every part's paths, on the final part -- instead of
// per-part (which would let part N's prune delete part N-1's files: last-part-wins data loss).
type PrunePlan struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

// NewPrunePlan returns an empty accumulator. Create one per multipart export set.
func NewPrunePlan() *PrunePlan {
	return &PrunePlan{paths: make(map[string]struct{})}
}

// Add records this part's projected file paths into the union.
func (p *PrunePlan) Add(paths []string) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.paths == nil {
		p.paths = make(map[string]struct{}, len(paths))
	}
	for _, path := range paths {
		p.paths[path] = struct{}{}
	}
}

// Union returns the sorted union of every path added so far.
func (p *PrunePlan) Union() []string {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]string, 0, len(p.paths))
	for path := range p.paths {
		out = append(out, path)
	}
	sort.Strings(out)

	return out
}

// FileExporter is implemented by git/gitlab backends that can write a projected layout tree in a
// single commit (ADR-0419). Backends that do not implement it fall back to single-document export.
type FileExporter interface {
	ExportFiles(ctx context.Context, files []git.FileEntry, opts git.ExportFilesOptions) error
}

// snapshotExport bundles the export closure with the representative object path used for commit
// context, fingerprinting, and metrics.
type snapshotExport struct {
	objectPath string
	run        func(ctx context.Context) error
}

// partSuffixRE matches the deterministic multipart object-path suffix (export.PartitionObjectPath).
var partSuffixRE = regexp.MustCompile(`\.part-\d+-of-\d+$`)

// baseInventoryName strips the .part-NNNN-of-NNNN suffix a multipart object path adds to the
// inventory name, recovering the per-set base identity for the manifest sidecar path.
func baseInventoryName(name string) string {
	return partSuffixRE.ReplaceAllString(name, "")
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
	prunePlan *PrunePlan,
) (snapshotExport, error) {
	if !isGitLayoutFamily(spec.Type) {
		return snapshotExport{
			objectPath: defaultObjectPath,
			run:        func(ctx context.Context) error { return backend.Export(ctx, envelope, defaultObjectPath) },
		}, nil
	}

	items, err := export.ItemsFromPayload(envelope)
	if err != nil {
		return snapshotExport{}, err
	}
	resourceExportMode, manifestKey := inferResourceLayoutHints(items)

	resolved := layout.Resolve(layout.ResolveInput{
		Spec:               spec,
		InventoryNamespace: invNS,
		InventoryName:      invName,
		Generation:         generation,
		ResourceExportMode: resourceExportMode,
		ManifestKey:        manifestKey,
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

	meta := export.EnvelopeMetaFromPayload(envelope)
	if !meta.ExportedAt.IsZero() {
		resolved.ExportedAt = meta.ExportedAt.UTC().Format(time.RFC3339)
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
	projectedPaths := make([]string, 0, len(files))
	for _, f := range files {
		gitFiles = append(gitFiles, git.FileEntry{Path: f.Path, Data: f.Data})
		projectedPaths = append(projectedPaths, f.Path)
	}

	opts := gitExportOpts(resolved.Prune, meta.PartIndex, meta.PartTotal, projectedPaths, prunePlan)

	gitFiles, err = appendSetManifest(resolved, gitFiles, &opts, meta.PartIndex, meta.PartTotal, prunePlan)
	if err != nil {
		return snapshotExport{}, err
	}

	return snapshotExport{
		objectPath: resolved.DocumentPath(),
		run:        func(ctx context.Context) error { return fileExporter.ExportFiles(ctx, gitFiles, opts) },
	}, nil
}

// gitExportOpts decides prune intent for one part of a (possibly multipart) layout export.
//
// The invariant: for a prune-bearing layout, prune must run EXACTLY ONCE, against the UNION of every
// part's projected paths, on the final part -- never per-part (which would let part N's prune delete
// part N-1's files: last-part-wins silent data loss). Single-part exports (partTotal <= 1) are
// unchanged. FAIL-SAFE: a multipart set with no accumulator suppresses prune entirely rather than
// risk a per-part prune.
func gitExportOpts(
	prune bool,
	partIndex, partTotal int,
	projectedPaths []string,
	plan *PrunePlan,
) git.ExportFilesOptions {
	opts := git.ExportFilesOptions{Prune: prune}
	if !prune || partTotal <= 1 {
		return opts
	}

	if plan == nil {
		// No shared accumulator: never prune per-part.
		opts.SuppressPrune = true

		return opts
	}

	plan.Add(projectedPaths)
	if partIndex >= partTotal {
		// Final part: prune once, keeping the union of every part's paths.
		opts.PruneKeepPaths = plan.Union()

		return opts
	}

	// Non-final part: suppress prune so it runs only on the final part.
	opts.SuppressPrune = true

	return opts
}

// appendSetManifest emits the per-export-set manifest sidecar (REL-02-FUP) on the FINAL part of a
// prune-bearing multipart YAML/layout export, giving YAML consumers the torn-set / stale-set marker
// the JSON ExportEnvelope already carries (ADR-0405/0419).
//
// It fires only for prune-bearing tree layouts (perResource/split) that are genuinely multipart --
// the case where parts occupy DISTINCT paths and a torn set is possible. It is deliberately NOT
// emitted for:
//   - single-part exports (partTotal <= 1): byte-compatible with today, no sidecar;
//   - document mode (Prune == false): all parts overwrite ONE path, a degenerate set with no distinct
//     part files to reconcile (partitioning math is out of scope; decide-and-log, ADR-0419);
//   - non-final parts / a nil accumulator: the manifest lists the accumulated UNION of every part's
//     paths, which is only complete on the final part.
//
// The manifest lists the union of projected DATA paths (from the shared accumulator) and its own path
// is appended to the prune keep set so the single union-prune preserves it -- survives a partial write
// and is replaced-not-orphaned on a new-generation re-export (its path is generation-stable).
func appendSetManifest(
	resolved layout.ResolvedLayout,
	gitFiles []git.FileEntry,
	opts *git.ExportFilesOptions,
	partIndex, partTotal int,
	plan *PrunePlan,
) ([]git.FileEntry, error) {
	if !resolved.Prune || partTotal <= 1 || partIndex < partTotal || plan == nil {
		return gitFiles, nil
	}

	// The per-part object path suffixes the inventory name with .part-NNNN-of-NNNN; the sidecar is
	// per-SET, so strip it back to the base inventory identity for a path stable across every part.
	setResolved := resolved
	setResolved.InventoryName = baseInventoryName(resolved.InventoryName)

	union := plan.Union()
	manifestPath := setResolved.SetManifestPath()

	// Fail loudly rather than silently overwrite a data file, mirroring the split-index collision
	// guard: a custom template must never render the manifest onto a projected resource path.
	for _, p := range union {
		if p == manifestPath {
			return nil, fmt.Errorf("layout collision: set-manifest path %q collides with a projected resource file", manifestPath)
		}
	}

	manifest := layout.BuildSetManifest(setResolved, partTotal, union)
	data, err := layout.MarshalSetManifest(manifest)
	if err != nil {
		return nil, err
	}

	gitFiles = append(gitFiles, git.FileEntry{Path: manifestPath, Data: data})
	opts.PruneKeepPaths = append(opts.PruneKeepPaths, manifestPath)

	return gitFiles, nil
}

func inferResourceLayoutHints(items []collect.Item) (bool, string) {
	if len(items) == 0 {
		return false, ""
	}

	manifestKey := ""
	for _, item := range items {
		key := inferManifestKey(item)
		if key == "" {
			return false, ""
		}
		if manifestKey == "" {
			manifestKey = key
			continue
		}
		if manifestKey != key {
			return false, ""
		}
	}

	return true, manifestKey
}

func inferManifestKey(item collect.Item) string {
	found := ""
	for key, value := range item.Attributes {
		obj, ok := value.(map[string]any)
		if !ok {
			continue
		}
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)
		if strings.TrimSpace(apiVersion) == "" || strings.TrimSpace(kind) == "" {
			continue
		}
		if strings.TrimSpace(item.Kind) != "" && !strings.EqualFold(strings.TrimSpace(kind), strings.TrimSpace(item.Kind)) {
			continue
		}
		if found != "" {
			return ""
		}
		found = key
	}

	return found
}
