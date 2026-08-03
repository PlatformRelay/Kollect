// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"encoding/json"
	"time"

	"github.com/platformrelay/kollect/internal/collect"
)

// EnvelopeMeta carries export envelope header fields for sink attribution (ADR-0415).
//
// PartIndex/PartTotal expose the multipart completeness marker (REL-02) so a
// consumer can validate that a reassembled set holds every index 1..PartTotal
// under a uniform Generation. Both are zero for a standalone single-part export.
type EnvelopeMeta struct {
	Generation int64
	Cluster    string
	ItemCount  int
	Checksum   string
	ExportedAt time.Time
	PartIndex  int
	PartTotal  int
}

// EnvelopeMetaFromPayload parses contract metadata from a marshalled export envelope.
func EnvelopeMetaFromPayload(payload []byte) EnvelopeMeta {
	var env collect.ExportEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return EnvelopeMeta{}
	}

	meta := EnvelopeMeta{
		Generation: env.Generation,
		Cluster:    env.Cluster,
		ItemCount:  env.ItemCount,
		Checksum:   env.Checksum,
		PartIndex:  env.PartIndex,
		PartTotal:  env.PartTotal,
	}
	if env.ExportedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, env.ExportedAt); err == nil {
			meta.ExportedAt = t.UTC()
		}
	}

	return meta
}
