// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
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
