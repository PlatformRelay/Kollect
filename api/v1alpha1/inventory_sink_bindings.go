// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// InventorySinkBinding ties a ref name to its sink family for export resolution (ADR-0414).
type InventorySinkBinding struct {
	Name   string
	Family string
	Ref    InventorySinkRef
}

// CollectInventorySinkBindings returns all family sink refs from an inventory spec.
func CollectInventorySinkBindings(spec *KollectInventorySpec) []InventorySinkBinding {
	return collectSinkBindings(
		spec.SnapshotSinkRefs, spec.DatabaseSinkRefs, spec.EventSinkRefs,
	)
}

// CollectClusterInventorySinkBindings returns family sink refs from cluster inventory spec.
func CollectClusterInventorySinkBindings(spec *KollectClusterInventorySpec) []InventorySinkBinding {
	return collectSinkBindings(
		spec.SnapshotSinkRefs, spec.DatabaseSinkRefs, spec.EventSinkRefs,
	)
}

func collectSinkBindings(
	snapshot, database, event InventorySinkRefList,
) []InventorySinkBinding {
	var out []InventorySinkBinding
	appendBindings := func(family string, refs InventorySinkRefList) {
		for _, ref := range refs {
			out = append(out, InventorySinkBinding{Name: ref.Name, Family: family, Ref: ref})
		}
	}
	appendBindings(SinkFamilySnapshot, snapshot)
	appendBindings(SinkFamilyDatabase, database)
	appendBindings(SinkFamilyEvent, event)
	return out
}

// TotalInventorySinkRefCount returns combined ref count across family lists.
func TotalInventorySinkRefCount(spec *KollectInventorySpec) int {
	if spec == nil {
		return 0
	}
	return len(spec.SnapshotSinkRefs) + len(spec.DatabaseSinkRefs) + len(spec.EventSinkRefs)
}

// TotalClusterInventorySinkRefCount returns combined ref count for cluster inventory.
func TotalClusterInventorySinkRefCount(spec *KollectClusterInventorySpec) int {
	if spec == nil {
		return 0
	}
	return len(spec.SnapshotSinkRefs) + len(spec.DatabaseSinkRefs) + len(spec.EventSinkRefs)
}

// AllInventorySinkRefLists returns ref lists for scope interval validation.
func AllInventorySinkRefLists(spec *KollectInventorySpec) []InventorySinkRefList {
	if spec == nil {
		return nil
	}
	return []InventorySinkRefList{spec.SnapshotSinkRefs, spec.DatabaseSinkRefs, spec.EventSinkRefs}
}

// AllClusterInventorySinkRefLists returns ref lists for cluster inventory interval validation.
func AllClusterInventorySinkRefLists(spec *KollectClusterInventorySpec) []InventorySinkRefList {
	if spec == nil {
		return nil
	}
	return []InventorySinkRefList{spec.SnapshotSinkRefs, spec.DatabaseSinkRefs, spec.EventSinkRefs}
}

// ClusterInventoryUsesClusterSinks is true when any family ref is set on cluster inventory.
func ClusterInventoryUsesClusterSinks(spec *KollectClusterInventorySpec) bool {
	return spec != nil && TotalClusterInventorySinkRefCount(spec) > 0
}
