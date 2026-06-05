// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package aggregate holds cross-target rollup helpers for Phase 4 (ADR-0033).
// Controllers still marshal namespace snapshots directly; this package documents
// identity keys and export-skip rules for richer multi-target aggregation.
package aggregate

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/konih/kollect/internal/collect"
)

// RowIdentity is the stable key for one collected row across targets.
// Hub merge extends this with cluster id (see internal/hub/merge.go).
type RowIdentity struct {
	TargetNamespace string
	TargetName      string
	Namespace       string
	Name            string
	UID             string
}

// IdentityFromItem derives a row identity from a collected item.
func IdentityFromItem(item collect.Item) RowIdentity {
	return RowIdentity{
		TargetNamespace: item.TargetNamespace,
		TargetName:      item.TargetName,
		Namespace:       item.Namespace,
		Name:            item.Name,
		UID:             item.UID,
	}
}

// ResourceUID is the cross-target collapse key when two targets watch the same object.
type ResourceUID struct {
	Namespace string
	UID       string
}

// ResourceKeyFromItem returns the resource-level dedupe key (namespace + uid).
func ResourceKeyFromItem(item collect.Item) ResourceUID {
	return ResourceUID{Namespace: item.Namespace, UID: item.UID}
}

// ContentHash returns a SHA-256 hex digest of an export payload.
func ContentHash(payload []byte) string {
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}

// ExportCoalesce tracks the last successful export fingerprint for debouncing.
type ExportCoalesce struct {
	LastGeneration int64
	LastHash       string
	LastExport     time.Time
}

// ShouldSkip reports whether an export can be coalesced within minInterval.
// Spec generation bumps and payload checksum changes bypass the interval
// (same rules as KollectInventory reconciler debounce).
func (c *ExportCoalesce) ShouldSkip(
	now time.Time,
	minInterval time.Duration,
	generation int64,
	payload []byte,
) bool {
	if c == nil {
		return false
	}

	if c.LastGeneration != generation {
		return false
	}

	hash := ContentHash(payload)
	if c.LastHash == "" || c.LastHash != hash {
		return false
	}

	if c.LastExport.IsZero() {
		return false
	}

	return now.Sub(c.LastExport) < minInterval
}

// RecordExport stores the fingerprint after a successful export.
func (c *ExportCoalesce) RecordExport(now time.Time, generation int64, payload []byte) {
	if c == nil {
		return
	}

	c.LastGeneration = generation
	c.LastHash = ContentHash(payload)
	c.LastExport = now
}

// DedupeMode selects how MergeRows collapses overlapping target rows.
type DedupeMode int

const (
	// DedupeKeepAll retains one row per (target, uid) — current inventory behavior.
	DedupeKeepAll DedupeMode = iota
	// DedupeByResourceUID keeps the last row per (namespace, uid) across targets.
	DedupeByResourceUID
)

// MergeRows returns items deduped per mode. Order is stable: later items win on conflict.
func MergeRows(items []collect.Item, mode DedupeMode) []collect.Item {
	if len(items) == 0 || mode == DedupeKeepAll {
		return append([]collect.Item(nil), items...)
	}

	seen := make(map[ResourceUID]int, len(items))
	out := make([]collect.Item, 0, len(items))

	for _, item := range items {
		key := ResourceKeyFromItem(item)
		if idx, ok := seen[key]; ok {
			out[idx] = item
			continue
		}

		seen[key] = len(out)
		out = append(out, item)
	}

	return out
}
