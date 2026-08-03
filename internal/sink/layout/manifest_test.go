// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/collect"
)

func multipartResolved() ResolvedLayout {
	return ResolvedLayout{
		Mode:               "perResource",
		Cluster:            "prod-west",
		InventoryNamespace: "team-a",
		InventoryName:      "api",
		Generation:         7,
		Extension:          ".yaml",
		ExportedAt:         "2026-08-04T00:00:00Z",
		Prune:              true,
	}
}

// TestBuildSetManifest_shapeAndVersioning asserts the per-set manifest carries the torn-set marker
// (generation + partTotal + per-part identifiers + the union of projected paths), self-identifies by
// kind, and is versioned by the ADR-0405 export schema version.
func TestBuildSetManifest_shapeAndVersioning(t *testing.T) {
	t.Parallel()

	paths := []string{
		"prod-west/team-a/Deployment/web.yaml",
		"prod-west/team-a/Deployment/api.yaml",
	}
	m := BuildSetManifest(multipartResolved(), 3, paths)

	if m.Kind != SetManifestKind {
		t.Fatalf("kind = %q, want %q (self-identifying for graceful-ignore)", m.Kind, SetManifestKind)
	}
	if m.SchemaVersion != collect.ExportSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q (ADR-0405)", m.SchemaVersion, collect.ExportSchemaVersion)
	}
	if m.Generation != 7 || m.PartTotal != 3 {
		t.Fatalf("generation/partTotal = %d/%d, want 7/3", m.Generation, m.PartTotal)
	}
	if len(m.Parts) != 3 || m.Parts[0] != 1 || m.Parts[2] != 3 {
		t.Fatalf("parts = %v, want dense 1..3", m.Parts)
	}
	if m.Namespace != "team-a" || m.Name != "api" || m.Cluster != "prod-west" {
		t.Fatalf("identity = %s/%s cluster %s, want team-a/api prod-west", m.Namespace, m.Name, m.Cluster)
	}
	// Paths are the union, deterministically sorted.
	if len(m.Paths) != 2 || m.Paths[0] != "prod-west/team-a/Deployment/api.yaml" {
		t.Fatalf("paths not sorted union: %v", m.Paths)
	}
}

// TestMarshalSetManifest_alwaysJSON asserts the sidecar is JSON regardless of sink format, so a
// consumer can always parse it, and that it round-trips.
func TestMarshalSetManifest_alwaysJSON(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 2, []string{"a.yaml", "b.yaml"})
	data, err := MarshalSetManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("manifest must end with a trailing newline")
	}

	var round SetManifest
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if round.Kind != SetManifestKind || round.PartTotal != 2 {
		t.Fatalf("round-trip lost fields: %+v", round)
	}
}

// TestSetManifestPath_deterministicDistinctExtension asserts the sidecar path is stable across parts
// (no generation/part placeholders) and ends in .manifest.json so a *.yaml globber skips it.
func TestSetManifestPath_deterministicDistinctExtension(t *testing.T) {
	t.Parallel()

	r := multipartResolved()
	got := r.SetManifestPath()
	if got != "inventory/team-a/api.manifest.json" {
		t.Fatalf("SetManifestPath = %q, want inventory/team-a/api.manifest.json", got)
	}
	if !strings.HasSuffix(got, ".manifest.json") {
		t.Fatalf("sidecar must be .manifest.json (distinct from .yaml), got %q", got)
	}
	// Stable across a generation bump (replace-not-orphan): path must NOT change with generation.
	r2 := r
	r2.Generation = 99
	if r2.SetManifestPath() != got {
		t.Fatal("sidecar path must be stable across generations so re-export replaces it in place")
	}
}

// TestVerifySet_complete asserts a fully-present set at the expected generation reads COMPLETE.
func TestVerifySet_complete(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 3, []string{"p1.yaml", "p2.yaml", "p3.yaml"})
	v := VerifySet(m, []string{"p1.yaml", "p2.yaml", "p3.yaml"}, 7)
	if !v.Complete {
		t.Fatalf("want COMPLETE, got %+v", v)
	}
}

// TestVerifySet_torn asserts a manifest declaring three parts but only two files on disk is detected
// INCOMPLETE (missing index) -- the torn-set edge.
func TestVerifySet_torn(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 3, []string{"p1.yaml", "p2.yaml", "p3.yaml"})
	v := VerifySet(m, []string{"p1.yaml", "p2.yaml"}, 7)
	if v.Complete {
		t.Fatal("torn set (2 of 3 files) must NOT be complete")
	}
	if len(v.Missing) != 1 || v.Missing[0] != "p3.yaml" {
		t.Fatalf("want Missing=[p3.yaml], got %v", v.Missing)
	}
}

// TestVerifySet_staleGeneration asserts a leftover gen-7 manifest verified against an expected gen-8
// (a torn export that never rewrote the manifest) is detected stale -- the stale-set edge. This is the
// ONLY stale signal for the default generation-independent template.
func TestVerifySet_staleGeneration(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 3, []string{"p1.yaml", "p2.yaml", "p3.yaml"})
	v := VerifySet(m, []string{"p1.yaml", "p2.yaml", "p3.yaml"}, 8)
	if v.Complete {
		t.Fatal("stale set (manifest gen 7, expected 8) must NOT be complete")
	}
	if !v.GenerationMismatch {
		t.Fatalf("want GenerationMismatch, got %+v", v)
	}
}

// TestVerifySet_staleOrphanExtras asserts that, for a generation-scoped template where prior-gen files
// linger beside the fresh set, the extras are surfaced as Stale and the set is not complete.
func TestVerifySet_staleOrphanExtras(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 2, []string{"g8/p1.yaml", "g8/p2.yaml"})
	// gen-7 leftovers linger beside the fresh gen-8 files.
	v := VerifySet(m, []string{"g8/p1.yaml", "g8/p2.yaml", "g7/p1.yaml"}, 8)
	if v.Complete {
		t.Fatal("orphan prior-gen extras must make the set not complete")
	}
	if len(v.Stale) != 1 || v.Stale[0] != "g7/p1.yaml" {
		t.Fatalf("want Stale=[g7/p1.yaml], got %v", v.Stale)
	}
}

// TestVerifySet_rawDirectoryListingIgnoresSidecar asserts the natural consumer call -- passing a raw
// listing of the managed directory, which INCLUDES the manifest sidecar itself -- still reads COMPLETE
// for a whole set (the control-file sidecar is not misclassified as a stale extra).
func TestVerifySet_rawDirectoryListingIgnoresSidecar(t *testing.T) {
	t.Parallel()

	r := multipartResolved()
	m := BuildSetManifest(r, 3, []string{"p1.yaml", "p2.yaml", "p3.yaml"})
	// What a consumer actually gets by listing the directory: every data file PLUS the sidecar.
	present := append([]string{}, m.Paths...)
	present = append(present, r.SetManifestPath())

	v := VerifySet(m, present, 7)
	if !v.Complete {
		t.Fatalf("raw listing including the sidecar must still read COMPLETE, got %+v", v)
	}
	if len(v.Stale) != 0 {
		t.Fatalf("the manifest sidecar must not be counted as a stale extra, got Stale=%v", v.Stale)
	}
}

// TestVerifySet_skipGenerationCheck asserts expectedGeneration=0 skips the generation gate (consumer
// with no independent generation signal), so a fully-present set still reads complete.
func TestVerifySet_skipGenerationCheck(t *testing.T) {
	t.Parallel()

	m := BuildSetManifest(multipartResolved(), 2, []string{"p1.yaml", "p2.yaml"})
	v := VerifySet(m, []string{"p1.yaml", "p2.yaml"}, 0)
	if !v.Complete || v.GenerationMismatch {
		t.Fatalf("expectedGeneration=0 must skip the generation gate, got %+v", v)
	}
}
