// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformrelay/kollect/internal/collect"
)

// Metadata is export envelope metadata carried alongside item rows (ADR-0405).
//
// PartIndex/PartTotal carry the multipart completeness marker (REL-02); they are
// zero for standalone single-part exports and set by PartitionEnvelopes for each
// shard of a torn-able multipart set.
type Metadata struct {
	Generation int64
	Cluster    string
	ExportedAt time.Time
	PartIndex  int
	PartTotal  int
}

// Envelope is the versioned inventory export document (alias for contract tests).
type Envelope = collect.ExportEnvelope

// MarshalEnvelope serializes items with contract metadata (ADR-0405).
func MarshalEnvelope(items []collect.Item, meta Metadata) ([]byte, error) {
	return collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: meta.Generation,
		Cluster:    meta.Cluster,
		ExportedAt: meta.ExportedAt,
		PartIndex:  meta.PartIndex,
		PartTotal:  meta.PartTotal,
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

// ItemsJSONFromEnvelope returns the canonical items JSON array from an export envelope.
func ItemsJSONFromEnvelope(payload []byte) ([]byte, error) {
	items, err := collect.ItemsFromExportPayload(payload)
	if err != nil {
		return nil, err
	}

	return json.Marshal(items)
}

// GenerationFromEnvelope reads generation metadata from a versioned export envelope.
func GenerationFromEnvelope(payload []byte) int64 {
	var env collect.ExportEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return 0
	}

	return env.Generation
}

// ValidateEnvelopeSchemaVersion rejects unsupported export contract versions.
func ValidateEnvelopeSchemaVersion(v string) error {
	v = NormalizeSchemaVersion(v)
	if v != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %q", v)
	}

	return nil
}
