// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/platformrelay/kollect/internal/collect"
)

// SetManifestKind self-identifies the per-export-set manifest sidecar so a YAML consumer globbing
// the managed directory can recognise and skip it regardless of file extension (ADR-0405/0419).
const SetManifestKind = "KollectExportSetManifest"

// DefaultSetManifestPathTemplate is the deterministic, per-*set* sidecar path. It is stable across
// every part of one multipart export (no {generation}/{partIndex} placeholders) so a re-export at a
// new generation REPLACES the manifest in place rather than orphaning it, and it always ends in
// .manifest.json -- a distinct extension from the .yaml/.ndjson data files so existing consumers that
// glob inventory/{ns}/*.yaml skip it (graceful-ignore, ADR-0405).
const DefaultSetManifestPathTemplate = "inventory/{namespace}/{name}.manifest.json"

// SetManifest is the per-export-set completeness sidecar that carries the multipart torn-set marker
// (generation + partTotal + the per-part identifiers + the union of projected data-file paths) beside
// the human-readable YAML layout projection, which itself carries no envelope metadata (ADR-0419).
//
// It is DISTINCT from layout.Index: Index is the per-projection split-mode sidecar (one projection's
// rows, for CI gating) whereas SetManifest spans EVERY part of one multipart set and is the
// authoritative record a consumer uses to detect a torn or stale set from the YAML output alone --
// the same guarantee JSON ExportEnvelope consumers already have (REL-02 / ADR-0405). The two coexist
// at different paths (.manifest.json vs the split index) and self-identify by kind, so a split-mode
// export that is ALSO multipart carries both without collision.
//
// It is always serialised as JSON (a machine-readable control file) regardless of the sink's
// serialization.format, and is versioned by SchemaVersion == collect.ExportSchemaVersion (ADR-0405,
// additive-only).
type SetManifest struct {
	Kind          string   `json:"kind"`
	SchemaVersion string   `json:"schemaVersion"`
	Cluster       string   `json:"cluster,omitempty"`
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	Generation    int64    `json:"generation"`
	PartTotal     int      `json:"partTotal"`
	Parts         []int    `json:"parts"`
	ExportedAt    string   `json:"exportedAt,omitempty"`
	Paths         []string `json:"paths"`
}

// SetManifestPath renders the deterministic per-set sidecar path for the resolved layout.
func (r ResolvedLayout) SetManifestPath() string {
	return renderInventoryPath(DefaultSetManifestPathTemplate, r)
}

// BuildSetManifest assembles the per-set manifest for a multipart export of partTotal parts whose
// union of projected data-file paths is paths. Set identity (generation, cluster, namespace, name,
// exportedAt) comes from the resolved layout; the per-part identifiers are the dense range
// 1..partTotal (every index a complete set must hold).
func BuildSetManifest(r ResolvedLayout, partTotal int, paths []string) SetManifest {
	parts := make([]int, 0, partTotal)
	for i := 1; i <= partTotal; i++ {
		parts = append(parts, i)
	}

	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)

	return SetManifest{
		Kind:          SetManifestKind,
		SchemaVersion: collect.ExportSchemaVersion,
		Cluster:       clusterOrDefault(r.Cluster),
		Namespace:     r.InventoryNamespace,
		Name:          r.InventoryName,
		Generation:    r.Generation,
		PartTotal:     partTotal,
		Parts:         parts,
		ExportedAt:    r.ExportedAt,
		Paths:         sorted,
	}
}

// MarshalSetManifest encodes the manifest as indented JSON with a trailing newline. The manifest is
// always JSON, independent of the sink's serialization.format.
func MarshalSetManifest(m SetManifest) ([]byte, error) {
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal set manifest: %w", err)
	}

	return append(out, '\n'), nil
}

// isControlSidecar reports whether a path is a per-set manifest control file rather than exported
// data, so a whole-directory listing does not misclassify the sidecar itself as a stale extra.
func isControlSidecar(p string) bool {
	return strings.HasSuffix(p, ".manifest.json")
}

// SetVerification is the outcome of validating an on-disk YAML set against its manifest (ADR-0405).
type SetVerification struct {
	// Complete is true only when no declared path is missing, no unexpected extra path is present,
	// and (when an expected generation was supplied) the manifest generation matches.
	Complete bool
	// Missing lists declared paths absent on disk -- a torn set (a part failed to persist).
	Missing []string
	// Stale lists on-disk paths not declared by the manifest -- orphans from a prior generation whose
	// path carried {generation} (generation-scoped templates); empty for the default in-place layout.
	Stale []string
	// GenerationMismatch is true when an expected generation was supplied and the manifest's
	// generation differs -- a stale set left by a torn export that never rewrote the manifest.
	GenerationMismatch bool
}

// VerifySet validates the on-disk paths of a multipart YAML set against its manifest, mirroring the
// JSON ExportEnvelope consumer rule (ADR-0405): every declared index present, count matches, and the
// generation is the one expected. Pass expectedGeneration = 0 to skip the generation check (e.g. when
// the consumer has no independent generation signal). Because the default layout templates are
// generation-INDEPENDENT (a new generation overwrites in place), the manifest's own generation field
// is the only stale signal for the default layout -- hence the expectedGeneration argument.
//
// present may be a raw listing of the managed directory: the manifest's data paths and any
// .manifest.json control-file sidecars are the expected members of a whole set, so control files are
// never counted as stale. Any OTHER on-disk path not declared by the manifest is surfaced as Stale --
// a prior-generation orphan under a generation-scoped path template (for the default in-place template
// there are none, and the stale signal is the generation field instead).
func VerifySet(m SetManifest, present []string, expectedGeneration int64) SetVerification {
	presentSet := make(map[string]struct{}, len(present))
	for _, p := range present {
		presentSet[p] = struct{}{}
	}

	declared := make(map[string]struct{}, len(m.Paths))
	var missing []string
	for _, p := range m.Paths {
		declared[p] = struct{}{}
		if _, ok := presentSet[p]; !ok {
			missing = append(missing, p)
		}
	}

	var stale []string
	for _, p := range present {
		if _, ok := declared[p]; ok {
			continue
		}
		if isControlSidecar(p) {
			// The manifest sidecar (and any sibling control file) is an expected member of the set,
			// not stale data -- a consumer that lists the whole directory includes it.
			continue
		}
		stale = append(stale, p)
	}

	sort.Strings(missing)
	sort.Strings(stale)

	genMismatch := expectedGeneration != 0 && m.Generation != expectedGeneration

	return SetVerification{
		Complete:           len(missing) == 0 && len(stale) == 0 && !genMismatch,
		Missing:            missing,
		Stale:              stale,
		GenerationMismatch: genMismatch,
	}
}
