// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ExportSchemaVersion is the inventory export envelope contract version (ADR-0405).
// Keep aligned with export.SchemaVersion.
const ExportSchemaVersion = "kollect.dev/v1alpha1"

// ExportMetadata is envelope metadata carried alongside item rows (ADR-0405).
type ExportMetadata struct {
	Generation int64
	Cluster    string
	ExportedAt time.Time
}

// ExportEnvelope is the versioned inventory export document written to state sinks.
type ExportEnvelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Checksum      string `json:"checksum"`
	Generation    int64  `json:"generation,omitempty"`
	ItemCount     int    `json:"itemCount"`
	ExportedAt    string `json:"exportedAt"`
	Cluster       string `json:"cluster,omitempty"`
	Items         []Item `json:"items"`
}

func itemCanonicalKey(item Item) string {
	var b strings.Builder
	b.Grow(128)
	b.WriteString(item.TargetNamespace)
	b.WriteByte(0)
	b.WriteString(item.TargetName)
	b.WriteByte(0)
	b.WriteString(item.Namespace)
	b.WriteByte(0)
	b.WriteString(item.Name)
	b.WriteByte(0)
	b.WriteString(item.UID)
	b.WriteByte(0)
	b.WriteString(item.Group)
	b.WriteByte(0)
	b.WriteString(item.Version)
	b.WriteByte(0)
	b.WriteString(item.Kind)

	return b.String()
}

func canonicalItems(items []Item) []Item {
	if len(items) == 0 {
		return []Item{}
	}

	out := slices.Clone(items)
	slices.SortFunc(out, func(a, b Item) int {
		return cmp.Compare(itemCanonicalKey(a), itemCanonicalKey(b))
	})

	return out
}

// MarshalExportEnvelope serializes items with contract metadata (ADR-0405).
func MarshalExportEnvelope(items []Item, meta ExportMetadata) ([]byte, error) {
	items = canonicalItems(items)

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal export items: %w", err)
	}

	exportedAt := meta.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}

	env := ExportEnvelope{
		SchemaVersion: ExportSchemaVersion,
		Checksum:      exportContentHash(itemsJSON),
		Generation:    meta.Generation,
		ItemCount:     len(items),
		ExportedAt:    exportedAt.UTC().Format(time.RFC3339Nano),
		Cluster:       meta.Cluster,
		Items:         items,
	}

	return json.Marshal(env)
}

// ItemsFingerprint returns a SHA-256 hex digest of the canonical items JSON.
func ItemsFingerprint(items []Item) (string, error) {
	items = canonicalItems(items)

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal items for fingerprint: %w", err)
	}

	return exportContentHash(itemsJSON), nil
}

// ItemsFromExportPayload decodes items from a versioned envelope or legacy bare array.
func ItemsFromExportPayload(payload []byte) ([]Item, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	trimmed := string(payload)
	if trimmed == "[]" || trimmed == "null" {
		return nil, nil
	}

	var env ExportEnvelope
	if err := json.Unmarshal(payload, &env); err == nil && env.SchemaVersion != "" {
		if env.SchemaVersion != ExportSchemaVersion {
			return nil, fmt.Errorf("unsupported schemaVersion %q", env.SchemaVersion)
		}

		return env.Items, nil
	}

	var items []Item
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, fmt.Errorf("decode export payload: %w", err)
	}

	return items, nil
}

func exportContentHash(payload []byte) string {
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}
