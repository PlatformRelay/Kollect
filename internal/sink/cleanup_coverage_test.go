// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

// cleanerBackend is a minimal snapshot backend that also implements
// ExportCleaner, letting the cleanup routing be exercised without a real sink.
type cleanerBackend struct {
	caps      cap.Capabilities
	deleteErr error
	deleted   []string

	deleteCalls int
}

func (b *cleanerBackend) Type() string                                 { return "cleaner" }
func (b *cleanerBackend) Capabilities() Capabilities                   { return b.caps }
func (b *cleanerBackend) Export(context.Context, []byte, string) error { return nil }

func (b *cleanerBackend) DeleteExport(context.Context, []string) ([]string, error) {
	b.deleteCalls++

	return b.deleted, b.deleteErr
}

// deleteSpec opts a sink spec into retraction (deletionPolicy Delete, ADR-0421).
func deleteSpec(sinkType string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{Type: sinkType, DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete}
}

func newCleanerRegistry(b *cleanerBackend) *Registry {
	reg := NewRegistry()
	reg.Register("cleaner", func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) {
		return b, nil
	})

	return reg
}

// A backend that cannot be acquired is a classified cleanup error, never a
// silent clean tombstone.
func TestRunCleanupExport_AcquireErrorIsClassified(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register("cleaner", func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) {
		return nil, errors.New("credentials missing")
	})

	_, err := RunCleanupExport(CleanupExportRequest{
		Ctx:      t.Context(),
		Registry: reg,
		SinkName: "cleaner-prod",
		SinkSpec: deleteSpec("cleaner"),
	})
	if err == nil {
		t.Fatal("acquire failure must surface as an error")
	}
}

// A cleaner's deletion error is classified (transient here) and surfaced, so
// cleanup retries rather than reporting a clean tombstone.
func TestRunCleanupExport_DeleteErrorIsClassified(t *testing.T) {
	t.Parallel()

	backend := &cleanerBackend{deleteErr: errors.New("connection reset")}
	reg := newCleanerRegistry(backend)

	_, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "cleaner-delete-error",
		SinkUID:    "uid-delete-error",
		SinkSpec:   deleteSpec("cleaner"),
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
	})
	if err == nil {
		t.Fatal("delete failure must surface as an error")
	}
}

// A successful cleaner on a plain snapshot sink is a clean tombstone.
func TestRunCleanupExport_CleanerSuccessIsCleaned(t *testing.T) {
	t.Parallel()

	backend := &cleanerBackend{deleted: []string{"inventory/team-a/inv.json"}}
	reg := newCleanerRegistry(backend)

	outcome, err := RunCleanupExport(CleanupExportRequest{
		Ctx:        t.Context(),
		Registry:   reg,
		SinkName:   "cleaner-success",
		SinkUID:    "uid-success",
		SinkSpec:   deleteSpec("cleaner"),
		ObjectPath: "inventory/team-a/inv.json",
		Inventory:  InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "team-a", Name: "inv"},
	})
	if err != nil {
		t.Fatalf("RunCleanupExport: %v", err)
	}
	if outcome != CleanupCleaned {
		t.Fatalf("outcome = %v, want CleanupCleaned", outcome)
	}
}

// A git layout sink in split mode addresses the index sidecar alongside the
// document and set manifest.
func TestCleanupCandidatePaths_IncludesIndexSidecar(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
		Layout: &kollectdevv1alpha1.LayoutSpec{
			Mode:  kollectdevv1alpha1.LayoutModeSplit,
			Index: &kollectdevv1alpha1.LayoutIndexSpec{PathTemplate: "inventory/{namespace}/{name}.index{extension}"},
		},
	}

	paths := cleanupCandidatePaths(spec, "team-a", "inv", 7)
	if len(paths) < 3 {
		t.Fatalf("paths = %v, want document, index and set manifest", paths)
	}
}

// dedupeStrings keeps the first occurrence order and drops repeats.
func TestDedupeStrings_DropsRepeats(t *testing.T) {
	t.Parallel()

	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedupeStrings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupeStrings = %v, want %v", got, want)
		}
	}
}
