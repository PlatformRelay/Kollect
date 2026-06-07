// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"testing"
	"time"
)

func TestEnvelopeMetaFromPayload(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"schemaVersion":"kollect.dev/v1alpha1",
		"checksum":"deadbeef",
		"generation":3,
		"itemCount":9,
		"exportedAt":"2026-06-07T08:15:30.123456789Z",
		"cluster":"eu-1",
		"items":[]
	}`)

	meta := EnvelopeMetaFromPayload(payload)
	if meta.Checksum != "deadbeef" || meta.Generation != 3 || meta.ItemCount != 9 || meta.Cluster != "eu-1" {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.ExportedAt.UTC().Format(time.RFC3339) != "2026-06-07T08:15:30Z" {
		t.Fatalf("exportedAt = %v", meta.ExportedAt)
	}
}
