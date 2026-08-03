// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"strings"
	"testing"
	"time"

	"github.com/platformrelay/kollect/internal/collect"
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

// multipartItems builds n items whose payloads are large enough to force
// per-part partitioning under a tight maxBytes.
func multipartItems(n int) []collect.Item {
	items := make([]collect.Item, 0, n)
	for i := range n {
		items = append(items, collect.Item{
			Namespace:  "apps",
			Name:       string(rune('a' + i)),
			Kind:       "Deployment",
			Version:    "v1",
			UID:        string(rune('a'+i)) + "-uid",
			Attributes: map[string]any{"payload": strings.Repeat(string(rune('a'+i)), 400)},
		})
	}

	return items
}

// TestPartitionEnvelopes_multiPartMarkersBakedIntoBytes is the REL-02 red→green
// driver: every persisted part's ENVELOPE BYTES must carry partIndex/partTotal +
// generation so a torn set is detectable from the payload alone, while each part
// still respects maxBytes.
func TestPartitionEnvelopes_multiPartMarkersBakedIntoBytes(t *testing.T) {
	t.Parallel()

	items := multipartItems(3)
	meta := Metadata{Generation: 9, ExportedAt: time.Unix(9, 0).UTC()}

	full, err := MarshalEnvelope(items, meta)
	if err != nil {
		t.Fatal(err)
	}
	maxBytes := int64(len(full) / 2)

	parts, err := PartitionEnvelopes(items, meta, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) < 2 {
		t.Fatalf("len(parts) = %d, want >= 2", len(parts))
	}

	for i := range parts {
		m := EnvelopeMetaFromPayload(parts[i].Envelope)
		if m.PartIndex != i+1 {
			t.Fatalf("part %d envelope partIndex = %d, want %d", i+1, m.PartIndex, i+1)
		}
		if m.PartTotal != len(parts) {
			t.Fatalf("part %d envelope partTotal = %d, want %d", i+1, m.PartTotal, len(parts))
		}
		if m.Generation != 9 {
			t.Fatalf("part %d envelope generation = %d, want 9", i+1, m.Generation)
		}
		if m.PartIndex != parts[i].Index || m.PartTotal != parts[i].Total {
			t.Fatalf("part %d struct/envelope marker mismatch: struct %d/%d envelope %d/%d",
				i+1, parts[i].Index, parts[i].Total, m.PartIndex, m.PartTotal)
		}
		if int64(len(parts[i].Envelope)) > maxBytes {
			t.Fatalf("part %d envelope = %d bytes, want <= %d", i+1, len(parts[i].Envelope), maxBytes)
		}
	}
}

// TestPartitionEnvelopes_singlePartMarkerless proves a standalone single-part
// export stays byte-identical to the legacy form (no part markers), so existing
// consumers are unaffected and absence-of-partTotal denotes "complete standalone".
func TestPartitionEnvelopes_singlePartMarkerless(t *testing.T) {
	t.Parallel()

	items := multipartItems(1)
	meta := Metadata{Generation: 4, ExportedAt: time.Unix(4, 0).UTC()}

	parts, err := PartitionEnvelopes(items, meta, 0) // unbounded → single part
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0].Index != 1 || parts[0].Total != 1 {
		t.Fatalf("part struct = index %d total %d, want 1/1", parts[0].Index, parts[0].Total)
	}

	m := EnvelopeMetaFromPayload(parts[0].Envelope)
	if m.PartIndex != 0 || m.PartTotal != 0 {
		t.Fatalf("single-part envelope must be markerless, got index %d total %d", m.PartIndex, m.PartTotal)
	}
	if s := string(parts[0].Envelope); strings.Contains(s, "partIndex") || strings.Contains(s, "partTotal") {
		t.Fatalf("single-part envelope must omit part fields, got %s", s)
	}
}

// TestPartitionEnvelopes_mixedGenerationDetectable proves a consumer can detect a
// stale/torn set: a part from generation N and a part from generation N-1 carry
// different Generation values under the same PartTotal, so reassembly can reject
// the mismatch instead of accepting a partial snapshot as complete.
func TestPartitionEnvelopes_mixedGenerationDetectable(t *testing.T) {
	t.Parallel()

	items := multipartItems(3)
	full, err := MarshalEnvelope(items, Metadata{Generation: 5})
	if err != nil {
		t.Fatal(err)
	}
	maxBytes := int64(len(full) / 2)

	genN, err := PartitionEnvelopes(items, Metadata{Generation: 5, ExportedAt: time.Unix(5, 0).UTC()}, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	genPrev, err := PartitionEnvelopes(items, Metadata{Generation: 4, ExportedAt: time.Unix(4, 0).UTC()}, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(genN) < 2 || len(genPrev) != len(genN) {
		t.Fatalf("part counts = %d / %d, want equal and >= 2", len(genN), len(genPrev))
	}

	m1 := EnvelopeMetaFromPayload(genN[0].Envelope)    // part 1 from generation 5
	m2 := EnvelopeMetaFromPayload(genPrev[1].Envelope) // part 2 from generation 4
	if m1.PartTotal != m2.PartTotal {
		t.Fatalf("part totals = %d / %d, want equal", m1.PartTotal, m2.PartTotal)
	}
	if m1.Generation == m2.Generation {
		t.Fatalf("generations must differ to detect a torn set, both = %d", m1.Generation)
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
