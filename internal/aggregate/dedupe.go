// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package aggregate holds cross-target rollup helpers for Phase 4 (ADR-0304).
// KollectClusterInventory uses MergeRows to collapse overlapping target rows; per-sink export
// coalescing/debounce lives in the controller (perSinkCoalesceTracker). Namespaced
// KollectInventory still marshals per-namespace snapshots directly.
package aggregate

import (
	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/digest"
)

// RowIdentity is the stable key for one collected row across targets.
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
	return digest.ContentHash(payload)
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

// DedupeModeFromSpec maps KollectClusterInventory.spec.dedupe to MergeRows mode (ADR-0305).
func DedupeModeFromSpec(spec *kollectdevv1alpha1.KollectClusterInventorySpec) DedupeMode {
	if spec == nil {
		return DedupeKeepAll
	}

	switch spec.Dedupe {
	case kollectdevv1alpha1.ClusterInventoryDedupeByResourceUID:
		return DedupeByResourceUID
	default:
		return DedupeKeepAll
	}
}
