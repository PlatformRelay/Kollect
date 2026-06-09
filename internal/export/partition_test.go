// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"strings"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestPartitionEnvelopes_emptySinglePart(t *testing.T) {
	t.Parallel()

	parts, err := PartitionEnvelopes(nil, Metadata{Generation: 1, ExportedAt: time.Unix(1, 0).UTC()}, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Total != 1 || parts[0].Index != 1 || parts[0].ItemCount != 0 {
		t.Fatalf("part[0] = %#v", parts[0])
	}
}

func TestPartitionEnvelopes_singlePartWithinLimit(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{Namespace: "apps", Name: "api", Kind: "Deployment", Version: "v1", UID: "u1"},
	}

	full, err := MarshalEnvelope(items, Metadata{Generation: 2, ExportedAt: time.Unix(2, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}

	parts, err := PartitionEnvelopes(items, Metadata{Generation: 2, ExportedAt: time.Unix(2, 0).UTC()}, int64(len(full)))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
}

func TestPartitionEnvelopes_multiPartBoundary(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{Namespace: "apps", Name: "api", Kind: "Deployment", Version: "v1", UID: "u1", Attributes: map[string]any{"payload": strings.Repeat("a", 280)}},
		{Namespace: "apps", Name: "web", Kind: "Deployment", Version: "v1", UID: "u2", Attributes: map[string]any{"payload": strings.Repeat("b", 280)}},
		{Namespace: "apps", Name: "jobs", Kind: "Deployment", Version: "v1", UID: "u3", Attributes: map[string]any{"payload": strings.Repeat("c", 280)}},
	}
	meta := Metadata{Generation: 7, ExportedAt: time.Unix(7, 0).UTC()}

	full, err := MarshalEnvelope(items, meta)
	if err != nil {
		t.Fatal(err)
	}
	maxBytes := int64(len(full) / 2)
	if maxBytes < 256 {
		maxBytes = 256
	}

	parts, err := PartitionEnvelopes(items, meta, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) < 2 {
		t.Fatalf("len(parts) = %d, want >= 2", len(parts))
	}

	composed := PartitionsChecksum(parts)
	if composed == "" {
		t.Fatal("composed checksum must be set")
	}

	for i := range parts {
		if parts[i].Total != len(parts) {
			t.Fatalf("part total = %d, want %d", parts[i].Total, len(parts))
		}
		if parts[i].Index != i+1 {
			t.Fatalf("part index = %d, want %d", parts[i].Index, i+1)
		}
		if int64(len(parts[i].Envelope)) > maxBytes {
			t.Fatalf("part %d envelope = %d bytes, want <= %d", i+1, len(parts[i].Envelope), maxBytes)
		}
	}
}

func TestPartitionObjectPath(t *testing.T) {
	t.Parallel()

	if got := PartitionObjectPath("inventory/team-a/api.json", 1, 1); got != "inventory/team-a/api.json" {
		t.Fatalf("single path = %q", got)
	}
	if got := PartitionObjectPath("inventory/team-a/api.json", 2, 3); got != "inventory/team-a/api.part-0002-of-0003.json" {
		t.Fatalf("multipart path = %q", got)
	}
}
