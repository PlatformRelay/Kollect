// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/platformrelay/kollect/internal/collect"
)

// EnvelopePartition is one bounded export envelope slice.
type EnvelopePartition struct {
	Index     int
	Total     int
	ItemCount int
	Checksum  string
	Envelope  []byte
}

// PartitionEnvelopes splits items into bounded envelope parts.
//
// A single-part result stays byte-identical to the legacy markerless form
// (backward compatible; absence of partTotal denotes a complete standalone
// document). A genuinely multipart result (REL-02) bakes partIndex/partTotal
// into every part's envelope bytes so a consumer can detect a torn or stale set
// from the payload alone. Size accounting reserves the marker width up front so
// each persisted part still respects maxBytes.
func PartitionEnvelopes(items []collect.Item, meta Metadata, maxBytes int64) ([]EnvelopePartition, error) {
	sorted, err := stableItems(items)
	if err != nil {
		return nil, err
	}

	groups, err := partitionItemGroups(sorted, meta, maxBytes)
	if err != nil {
		return nil, err
	}

	total := len(groups)
	parts := make([]EnvelopePartition, 0, total)
	for i, group := range groups {
		partMeta := meta
		// Emit the completeness marker only for genuinely multipart sets; a lone
		// part stays markerless so existing single-document consumers are
		// unaffected.
		if total > 1 {
			partMeta.PartIndex = i + 1
			partMeta.PartTotal = total
		}

		payload, marshalErr := MarshalEnvelope(group, partMeta)
		if marshalErr != nil {
			return nil, marshalErr
		}
		parts = append(parts, EnvelopePartition{
			Index:     i + 1,
			Total:     total,
			ItemCount: len(group),
			Checksum:  EnvelopeMetaFromPayload(payload).Checksum,
			Envelope:  payload,
		})
	}

	return parts, nil
}

// partitionItemGroups splits sorted items into the groups that will each become
// one bounded envelope part. The single-vs-multipart decision is measured on the
// markerless envelope so a standalone part stays byte-identical to the legacy
// form; once multipart, per-candidate sizing reserves the completeness-marker
// width (an upper bound on the real partIndex/partTotal digits) so the marked
// part that is actually persisted still fits within maxBytes.
func partitionItemGroups(sorted []collect.Item, meta Metadata, maxBytes int64) ([][]collect.Item, error) {
	full, err := MarshalEnvelope(sorted, meta)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 || int64(len(full)) <= maxBytes {
		return [][]collect.Item{sorted}, nil
	}
	if len(sorted) == 0 {
		return [][]collect.Item{{}}, nil
	}

	// Reserve marker width: the real partTotal never exceeds len(sorted) (worst
	// case one item per part) and partIndex never exceeds partTotal, so sizing
	// with both fields set to len(sorted) guarantees measured >= persisted.
	measureMeta := meta
	measureMeta.PartIndex = len(sorted)
	measureMeta.PartTotal = len(sorted)

	var groups [][]collect.Item
	current := make([]collect.Item, 0, len(sorted))

	for i := range sorted {
		candidate := append(current, sorted[i]) //nolint:gocritic // intentional grow-or-reset below
		payload, marshalErr := MarshalEnvelope(candidate, measureMeta)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if int64(len(payload)) <= maxBytes {
			current = candidate
			continue
		}

		if len(current) == 0 {
			return nil, fmt.Errorf("single item export envelope exceeds maxExportBytes (%d)", maxBytes)
		}

		// Clone before flushing: groups are marshalled after this loop, so they
		// must not alias a backing array that later appends could mutate.
		groups = append(groups, slices.Clone(current))
		current = []collect.Item{sorted[i]}

		single, marshalErr := MarshalEnvelope(current, measureMeta)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if int64(len(single)) > maxBytes {
			return nil, fmt.Errorf("single item export envelope exceeds maxExportBytes (%d)", maxBytes)
		}
	}

	if len(current) > 0 {
		groups = append(groups, slices.Clone(current))
	}

	return groups, nil
}

// PartitionsChecksum returns a stable digest over part checksums.
func PartitionsChecksum(parts []EnvelopePartition) string {
	sum := sha256.New()
	for i := range parts {
		_, _ = sum.Write([]byte(parts[i].Checksum))
		_, _ = sum.Write([]byte{'\n'})
	}

	return hex.EncodeToString(sum.Sum(nil))
}

// PartitionObjectPath appends a deterministic part suffix for multipart exports.
func PartitionObjectPath(baseObjectPath string, index, total int) string {
	if total <= 1 {
		return baseObjectPath
	}

	dir, file := path.Split(baseObjectPath)
	dot := strings.LastIndex(file, ".")
	if dot <= 0 {
		return path.Join(dir, fmt.Sprintf("%s.part-%04d-of-%04d", file, index, total))
	}

	name := file[:dot]
	ext := file[dot:]

	return path.Join(dir, fmt.Sprintf("%s.part-%04d-of-%04d%s", name, index, total, ext))
}

func stableItems(items []collect.Item) ([]collect.Item, error) {
	if len(items) == 0 {
		return []collect.Item{}, nil
	}

	type keyedItem struct {
		item collect.Item
		key  string
	}

	keyed := make([]keyedItem, 0, len(items))
	for i := range items {
		raw, err := json.Marshal(items[i])
		if err != nil {
			return nil, fmt.Errorf("marshal item sort key: %w", err)
		}
		keyed = append(keyed, keyedItem{item: items[i], key: string(raw)})
	}

	slices.SortFunc(keyed, func(a, b keyedItem) int {
		return cmp.Compare(a.key, b.key)
	})

	out := make([]collect.Item, 0, len(keyed))
	for i := range keyed {
		out = append(out, keyed[i].item)
	}

	return out, nil
}
