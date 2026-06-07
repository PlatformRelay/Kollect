// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"github.com/konih/kollect/internal/collect"
)

// Index is the split-mode index sidecar: an envelope summary plus the row path list (ADR-0419).
// It carries metadata for CI gating without the full per-resource payloads.
type Index struct {
	SchemaVersion string   `json:"schemaVersion"`
	ItemCount     int      `json:"itemCount"`
	Checksum      string   `json:"checksum,omitempty"`
	ExportedAt    string   `json:"exportedAt,omitempty"`
	Cluster       string   `json:"cluster,omitempty"`
	Paths         []string `json:"paths"`
}

// buildIndex assembles the split-mode index for the given items and rendered paths.
func buildIndex(r ResolvedLayout, items []collect.Item, paths []string) Index {
	checksum, _ := collect.ItemsFingerprint(items)

	return Index{
		SchemaVersion: collect.ExportSchemaVersion,
		ItemCount:     len(items),
		Checksum:      checksum,
		ExportedAt:    r.ExportedAt,
		Cluster:       clusterOrDefault(r.Cluster),
		Paths:         paths,
	}
}
