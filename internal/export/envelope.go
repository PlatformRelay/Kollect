// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"fmt"
	"time"

	"github.com/konih/kollect/internal/collect"
)

// Metadata is export envelope metadata carried alongside item rows (ADR-0405).
type Metadata struct {
	Generation int64
	Cluster    string
	ExportedAt time.Time
}

// Envelope is the versioned inventory export document (alias for contract tests).
type Envelope = collect.ExportEnvelope

// MarshalEnvelope serializes items with contract metadata (ADR-0405).
func MarshalEnvelope(items []collect.Item, meta Metadata) ([]byte, error) {
	return collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: meta.Generation,
		Cluster:    meta.Cluster,
		ExportedAt: meta.ExportedAt,
	})
}

// ItemsFingerprint returns a SHA-256 hex digest of the canonical items JSON.
func ItemsFingerprint(items []collect.Item) (string, error) {
	return collect.ItemsFingerprint(items)
}

// ItemsFromPayload decodes items from a versioned envelope or legacy bare array.
func ItemsFromPayload(payload []byte) ([]collect.Item, error) {
	return collect.ItemsFromExportPayload(payload)
}

// ValidateEnvelopeSchemaVersion rejects unsupported export contract versions.
func ValidateEnvelopeSchemaVersion(v string) error {
	v = NormalizeSchemaVersion(v)
	if v != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %q", v)
	}

	return nil
}
