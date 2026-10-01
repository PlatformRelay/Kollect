// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"errors"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// countingRegistry registers a factory for sinkType that counts how often the
// backend is built, so a test can prove cleanup never contacted the backend.
func countingRegistry(sinkType string, backend Backend, built *int) *Registry {
	reg := NewRegistry()
	reg.Register(sinkType, func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) {
		*built++
		if backend == nil {
			return nil, errors.New("credentials revoked")
		}

		return backend, nil
	})

	return reg
}

// ADR-0421: a snapshot sink created without spec.deletionPolicy retains the
// inventory's exported objects. Cleanup must not touch them and must not even
// build the backend, so a revoked credential cannot wedge a Retain deletion.
func TestRunCleanupExport_DefaultPolicyRetainsWithoutBackendContact(t *testing.T) {
	t.Parallel()

	for _, sinkType := range []string{
		kollectdevv1alpha1.SnapshotSinkTypeGit,
		kollectdevv1alpha1.SnapshotSinkTypeGitLab,
		kollectdevv1alpha1.SnapshotSinkTypeS3,
		kollectdevv1alpha1.SnapshotSinkTypeGCS,
	} {
		t.Run(sinkType, func(t *testing.T) {
			t.Parallel()

			built := 0
			reg := countingRegistry(sinkType, nil, &built)

			outcome, err := RunCleanupExport(CleanupExportRequest{
				Ctx:        t.Context(),
				Registry:   reg,
				SinkName:   "retain-" + sinkType,
				SinkUID:    "uid-retain",
				SinkSpec:   kollectdevv1alpha1.KollectSinkSpec{Type: sinkType},
				ObjectPath: "inventory/team-a/inv.json",
				Generation: 3,
			})
			if err != nil {
				t.Fatalf("RunCleanupExport: %v", err)
			}
			if outcome != CleanupRetainedByPolicy {
				t.Fatalf("outcome = %v, want CleanupRetainedByPolicy", outcome)
			}
			if built != 0 {
				t.Fatalf("backend built %d times under Retain, want 0 (no backend contact)", built)
			}
		})
	}
}

// An explicit Retain behaves like the default, also for a backend that would
// otherwise retract (the cleaner is acquired only through a non-snapshot type).
func TestRunCleanupExport_ExplicitRetainNeverCallsDeleteExport(t *testing.T) {
	t.Parallel()

	backend := &cleanerBackend{deleted: []string{"inventory/team-a/inv.json"}}
	reg := newCleanerRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:      t.Context(),
		Registry: reg,
		SinkName: "cleaner-retain",
		SinkUID:  "uid-cleaner-retain",
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:           "cleaner",
			DeletionPolicy: kollectdevv1alpha1.DeletionPolicyRetain,
		},
		ObjectPath: "inventory/team-a/inv.json",
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupRetainedByPolicy {
		t.Fatalf("outcome = %v, want CleanupRetainedByPolicy", outcome)
	}
	if backend.deleteCalls != 0 {
		t.Fatalf("DeleteExport called %d times under Retain, want 0", backend.deleteCalls)
	}
}

// With Delete the retraction of K-28 runs: DeleteExport receives the candidates.
func TestRunCleanupExport_DeletePolicyRetracts(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:               t.Context(),
		Registry:          reg,
		SinkName:          "git-delete",
		SinkUID:           "uid-git-delete",
		SinkSpec:          defaultGitSinkSpec(),
		ObjectPath:        "inventory/team-a/inv.json",
		Generation:        7,
		LastExportedPaths: []string{"inventory/team-a/inv.yaml"},
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupCleaned {
		t.Fatalf("outcome = %v, want CleanupCleaned", outcome)
	}
	if len(backend.deleteCalls) != 1 {
		t.Fatalf("DeleteExport calls = %d, want 1", len(backend.deleteCalls))
	}
}

// MR-09: a cluster inventory X and a namespaced inventory X in namespace
// "cluster" render the same export identity. Retracting one would delete the
// other's export, so a Delete-policy cleanup whose identity is shared skips the
// retraction and announces it instead.
func TestRunCleanupExport_SharedIdentitySkipsRetraction(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:                  t.Context(),
		Registry:             reg,
		SinkName:             "git-shared",
		SinkUID:              "uid-git-shared",
		SinkSpec:             defaultGitSinkSpec(),
		ObjectPath:           "inventory/cluster/inv.json",
		Generation:           2,
		SharedExportIdentity: true,
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupRetainedSharedIdentity {
		t.Fatalf("outcome = %v, want CleanupRetainedSharedIdentity", outcome)
	}
	if len(backend.deleteCalls) != 0 {
		t.Fatalf("DeleteExport called %d times for a shared identity, want 0", len(backend.deleteCalls))
	}
}

// MR-01: an inventory that exported before lastExportPaths existed has no
// recorded evidence. A deletion that addressed the candidates still cannot
// prove that nothing else was written, so it must announce retention rather
// than claim a clean tombstone.
func TestRunCleanupExport_UnrecordedPastExportIsNotClean(t *testing.T) {
	t.Parallel()

	backend := &evidenceCleanerBackend{}
	reg := newEvidenceRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:                   t.Context(),
		Registry:              reg,
		SinkName:              "git-pre-upgrade",
		SinkUID:               "uid-git-pre-upgrade",
		SinkSpec:              defaultGitSinkSpec(),
		ObjectPath:            "inventory/team-a/inv.json",
		Generation:            4,
		ExportPathsUnrecorded: true,
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupRetained {
		t.Fatalf("outcome = %v, want CleanupRetained (no evidence for a past export)", outcome)
	}
	if len(backend.deleteCalls) != 1 {
		t.Fatalf("DeleteExport calls = %d, want 1 (retraction still runs)", len(backend.deleteCalls))
	}
}

// Relational sinks prune by the empty export regardless of deletionPolicy: the
// policy only governs snapshot objects.
func TestRunCleanupExport_RelationalPruneIgnoresPolicy(t *testing.T) {
	t.Parallel()

	backend := &cleanerBackend{caps: Capabilities{SupportsDelete: true}}
	reg := newCleanerRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "cleaner-relational",
		SinkUID:    "uid-relational",
		SinkSpec:   kollectdevv1alpha1.KollectSinkSpec{Type: "cleaner"},
		ObjectPath: "inventory/team-a/inv.json",
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupPruned {
		t.Fatalf("outcome = %v, want CleanupPruned", outcome)
	}
}
