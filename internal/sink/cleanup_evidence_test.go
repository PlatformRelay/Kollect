// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// evidenceCleanerBackend is a git-family ExportCleaner stub whose delete report
// is configurable, letting the recorded-path retention decision be exercised
// without a real sink. respond maps the requested candidate paths to the paths
// reported as actually deleted (the ExportCleaner return the cleanup decision
// must consume); a nil respond echoes the candidates.
type evidenceCleanerBackend struct {
	respond func(paths []string) []string

	deleteCalls  [][]string
	releaseCalls []git.ReleaseOptions
}

func (b *evidenceCleanerBackend) Type() string {
	return kollectdevv1alpha1.SnapshotSinkTypeGit
}

func (b *evidenceCleanerBackend) Capabilities() Capabilities {
	return SnapshotStoreCapabilities()
}

func (b *evidenceCleanerBackend) Export(context.Context, []byte, string) error { return nil }

func (b *evidenceCleanerBackend) DeleteExport(_ context.Context, paths []string) ([]string, error) {
	cp := append([]string(nil), paths...)
	b.deleteCalls = append(b.deleteCalls, cp)

	if b.respond != nil {
		return b.respond(cp), nil
	}

	return cp, nil
}

// ReleaseExport records the release and retracts through DeleteExport unless the files are kept,
// so the evidence assertions below see the same deletions as before (ADR-0422).
func (b *evidenceCleanerBackend) ReleaseExport(ctx context.Context, paths []string, opts git.ReleaseOptions) ([]string, error) {
	b.releaseCalls = append(b.releaseCalls, opts)
	if opts.KeepFiles {
		return nil, nil
	}

	return b.DeleteExport(ctx, paths)
}

func newEvidenceRegistry(backend *evidenceCleanerBackend) *Registry {
	reg := NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGit, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext,
	) (Backend, error) {
		return backend, nil
	})

	return reg
}

// defaultGitSinkSpec is the common default git sink: snapshot store, YAML
// serialization, no explicit layout.mode, default path templates — opted into
// retraction (deletionPolicy Delete, ADR-0421), which these tests exercise.
func defaultGitSinkSpec() kollectdevv1alpha1.KollectSinkSpec {
	return deleteSpec(kollectdevv1alpha1.SnapshotSinkTypeGit)
}

// TestRunCleanupExport_DefaultGitDocumentFullyRetractedIsClean is the
// false-positive regression (K-28 follow-up): a default git sink (no explicit
// layout.mode) whose recorded export was the plain document and whose deletion
// retracted every candidate must NOT announce CleanupRetained. The pre-fix
// decision announced retention on every such deletion because the
// implicit-tree heuristic could not be ruled out — alert noise that trained
// operators to ignore the warning.
func TestRunCleanupExport_DefaultGitDocumentFullyRetractedIsClean(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	// Recorded by the last successful export: the resolved git document path
	// (YAML by default). The deletion retracts every candidate (stub echoes the
	// requested paths as deleted), so nothing recorded survives.
	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:               t.Context(),
		Registry:          reg,
		SinkName:          "git-fp-clean",
		SinkUID:           "uid-git-fp-clean",
		SinkSpec:          defaultGitSinkSpec(),
		ObjectPath:        "inventory/team-a/inv.json",
		Inventory:         InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
		Generation:        7,
		LastExportedPaths: []string{"inventory/team-a/inv.yaml"},
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupCleaned {
		t.Fatalf("outcome = %v, want CleanupCleaned (false-positive CleanupRetained on a fully retracted default git sink)", outcome)
	}
}

// An inventory that never successfully exported (no recorded paths) and whose
// candidates matched nothing has nothing to retain: a clean outcome without a
// retention announcement.
func TestRunCleanupExport_DefaultGitNeverExportedIsClean(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{respond: func([]string) []string { return nil }}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "git-never-exported",
		SinkUID:    "uid-git-never-exported",
		SinkSpec:   defaultGitSinkSpec(),
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
		Generation: 7,
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupCleaned {
		t.Fatalf("outcome = %v, want CleanupCleaned for a sink that never received an export", outcome)
	}
}

// K-28: with layout.mode unset, an auto-upgraded per-resource tree writes files
// no document-side candidate addresses. The recorded export paths make that
// shape visible at deletion time and the retention announcement stays honest —
// without the recorded evidence, tree files would be silently retained.
func TestRunCleanupExport_RecordedTreePathsAnnounceRetention(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "git-auto-upgraded-tree",
		SinkUID:    "uid-git-auto-upgraded-tree",
		SinkSpec:   defaultGitSinkSpec(),
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
		Generation: 7,
		LastExportedPaths: []string{
			"inventory/team-a/inv.yaml",
			"resources/apps/core-1.yaml",
			"resources/apps/core-2.yaml",
		},
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupRetained {
		t.Fatalf("outcome = %v, want CleanupRetained for a recorded tree-shaped export", outcome)
	}
}

// The silent false-clean tombstone (K-28): when the sink's pathTemplate or
// serialization format changed after the last export, the recorded paths are no
// longer addressable by the candidates — the deletion removes nothing and the
// old objects stay behind. The recorded evidence must announce retention
// instead of the pre-fix silent CleanupCleaned. Both cases pin an explicit
// document layout: there the pre-fix code had no implicit-tree doubt to hide
// behind and reported a clean tombstone over retained data.
func TestRunCleanupExport_RecordedPathOutsideCandidatesAnnouncesRetention(t *testing.T) {
	t.Parallel()

	documentGitSpec := func() kollectdevv1alpha1.KollectSinkSpec {
		return kollectdevv1alpha1.KollectSinkSpec{
			Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
			DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
			Layout:         &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument},
		}
	}

	cases := map[string]struct {
		spec              kollectdevv1alpha1.KollectSinkSpec
		lastExportedPaths []string
	}{
		// The template changed after the export wrote the old path: candidates
		// render the new template, the old object is behind them.
		"path template changed": {
			spec:              documentGitSpec(),
			lastExportedPaths: []string{"inventory/team-a/old-place/inv.yaml"},
		},
		// serialization.format: json wrote the .json document; the spec now
		// resolves to the YAML default, so candidates render .yaml.
		"serialization format changed": {
			spec:              documentGitSpec(),
			lastExportedPaths: []string{"inventory/team-a/inv.json"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Nothing matched the candidates: nothing existed at the new
			// addresses, which is exactly why the old objects stay behind.
			backend := &evidenceCleanerBackend{respond: func([]string) []string { return nil }}
			reg := newEvidenceRegistry(backend)

			outcome, err := RunCleanupExport(CleanupExportRequest{
				Ctx:               t.Context(),
				Registry:          reg,
				SinkName:          "git-changed-config",
				SinkUID:           types.UID("uid-git-changed-config-" + name),
				SinkSpec:          tc.spec,
				ObjectPath:        "inventory/team-a/inv.json",
				Inventory:         InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
				Generation:        7,
				LastExportedPaths: tc.lastExportedPaths,
			})
			if err != nil {
				t.Fatalf("RunCleanupExport: %v", err)
			}
			if outcome != CleanupRetained {
				t.Fatalf("outcome = %v, want CleanupRetained (silent false-clean tombstone)", outcome)
			}
		})
	}
}

// The deleted-paths seam is live: part siblings the backend's matchers swept
// beyond the exact candidates count as addressed, so a multipart document
// export whose recorded part paths were all removed by the matcher sweep is a
// clean tombstone even though the candidates listed only the base path.
func TestRunCleanupExport_RecordedPartSiblingsAddressedByDeletedReport(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{
		// A real git backend sweeps each candidate's deterministic
		// .part-NNNN-of-NNNN siblings and reports every removed path; emulate
		// that sweep so the deleted report carries the part paths.
		respond: func(paths []string) []string {
			return append(
				append([]string(nil), paths...),
				"inventory/team-a/inv.part-0000-of-0002.yaml",
				"inventory/team-a/inv.part-0001-of-0002.yaml",
			)
		},
	}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "git-part-siblings",
		SinkUID:    "uid-git-part-siblings",
		SinkSpec:   defaultGitSinkSpec(),
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
		Generation: 7,
		LastExportedPaths: []string{
			"inventory/team-a/inv.part-0000-of-0002.yaml",
			"inventory/team-a/inv.part-0001-of-0002.yaml",
		},
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupCleaned {
		t.Fatalf("outcome = %v, want CleanupCleaned (the deleted report proves the part paths were removed)", outcome)
	}
}

// Recorded paths only describe the LAST export: an explicit per-resource
// layout is announced structurally even with no recorded evidence, because its
// tree files interleave with other inventories' trees under shared templates.
func TestRunCleanupExport_ExplicitTreeModeAnnouncesRetentionWithoutRecordedState(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
		DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
		Layout:         &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "git-explicit-tree",
		SinkUID:    "uid-git-explicit-tree",
		SinkSpec:   spec,
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
		Generation: 7,
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupRetained {
		t.Fatalf("outcome = %v, want CleanupRetained for an explicit per-resource layout", outcome)
	}
}
