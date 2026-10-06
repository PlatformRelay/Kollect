// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dedupe mode values for KollectClusterInventory.spec.dedupe (ADR-0305).
const (
	ClusterInventoryDedupeKeepAll       = "keepAll"
	ClusterInventoryDedupeByResourceUID = "byResourceUID"
)

// KollectClusterInventorySpec defines platform-wide rollup across KollectClusterTarget objects
// (ADR-0201). Reconciled by KollectClusterInventoryReconciler.
type KollectClusterInventorySpec struct {
	// profileRef optionally overrides the rollup extraction schema with a namespaced
	// KollectProfile by name and namespace (ADR-0208). namespace is required when set.
	// +optional
	ProfileRef *NamespacedObjectReference `json:"profileRef,omitempty"`

	// targetRefs lists KollectClusterTarget names to aggregate.
	// When empty, all cluster targets matching targetSelector are included; when targetSelector is
	// also empty, all KollectClusterTarget objects contribute.
	// +optional
	TargetRefs []string `json:"targetRefs,omitempty"`

	// targetSelector narrows which KollectClusterTarget objects contribute when targetRefs is empty.
	// +optional
	TargetSelector *metav1.LabelSelector `json:"targetSelector,omitempty"`

	// namespaceSelector restricts rollup to namespaces matching the selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// namespaces explicitly lists namespace names that may contribute to the rollup.
	// When set with namespaceSelector, the effective rollup scope is their intersection.
	// +listType=set
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:items:MaxLength=63
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// snapshotSinkRefs lists namespaced KollectSnapshotSink refs (ADR-0414, ADR-0208).
	// Each ref resolves in its own namespace, or spec.sinkNamespace when omitted.
	// +optional
	SnapshotSinkRefs InventorySinkRefList `json:"snapshotSinkRefs,omitempty"`

	// databaseSinkRefs lists namespaced KollectDatabaseSink refs (ADR-0414, ADR-0208).
	// Each ref resolves in its own namespace, or spec.sinkNamespace when omitted.
	// +optional
	DatabaseSinkRefs InventorySinkRefList `json:"databaseSinkRefs,omitempty"`

	// eventSinkRefs lists namespaced KollectEventSink refs (ADR-0414, ADR-0208).
	// Each ref resolves in its own namespace, or spec.sinkNamespace when omitted.
	// +optional
	EventSinkRefs InventorySinkRefList `json:"eventSinkRefs,omitempty"`

	// sinkNamespace is the default namespace for family sink refs that omit a namespace.
	// +kubebuilder:default="kollect-system"
	// +optional
	SinkNamespace string `json:"sinkNamespace,omitempty"`

	// exportMinInterval is the minimum time between identical exports for this inventory.
	// It debounces identical payloads only: material changes (payload checksum or spec
	// generation) always export immediately, regardless of the interval. Zero means
	// material-change only (no periodic re-export of identical payloads).
	// +kubebuilder:default="30s"
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`

	// dedupe selects how overlapping target rows collapse on export (ADR-0305).
	// +kubebuilder:validation:Enum=keepAll;byResourceUID
	// +kubebuilder:default=keepAll
	// +optional
	Dedupe string `json:"dedupe,omitempty"`

	// suspend pauses reconciliation when set to true.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// KollectClusterInventoryStatus holds rollup export status for platform operators.
type KollectClusterInventoryStatus struct {
	// conditions represent the current state of the KollectClusterInventory resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed by a controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// itemCount is the number of inventory rows in the last rollup.
	// +optional
	ItemCount int `json:"itemCount,omitempty"`

	// targetCount is the number of KollectClusterTarget objects included in the last rollup.
	// +optional
	TargetCount int `json:"targetCount,omitempty"`

	// namespaceShardCount is the number of namespace shards composed into the last rollup.
	// +optional
	NamespaceShardCount int `json:"namespaceShardCount,omitempty"`

	// namespaceShards holds per-namespace shard metadata used to compose the rollup.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	NamespaceShards []InventoryNamespaceShardStatus `json:"namespaceShards,omitempty"`

	// lastExportTime is the timestamp of the last successful export across all sinks.
	// +optional
	LastExportTime *metav1.Time `json:"lastExportTime,omitempty"`

	// sinkExports holds per-sink export timestamps and conditions.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=20
	SinkExports []InventorySinkExportStatus `json:"sinkExports,omitempty"`
}

// InventoryNamespaceShardStatus tracks one namespace shard contributing to a rollup.
type InventoryNamespaceShardStatus struct {
	// namespace is the namespace name represented by this shard.
	// +required
	Namespace string `json:"namespace"`

	// itemCount is the number of rows in this namespace shard after dedupe.
	// +optional
	ItemCount int `json:"itemCount,omitempty"`

	// targetCount is the number of targets that contributed rows to this shard.
	// +optional
	TargetCount int `json:"targetCount,omitempty"`

	// checksum is the canonical checksum of shard rows.
	// +optional
	Checksum string `json:"checksum,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kcinv
// +kubebuilder:storageversion

// KollectClusterInventory rolls up cluster targets for platform operators.
type KollectClusterInventory struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectClusterInventory
	// +required
	Spec KollectClusterInventorySpec `json:"spec"`

	// status defines the observed state of KollectClusterInventory
	// +optional
	Status KollectClusterInventoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectClusterInventoryList contains a list of KollectClusterInventory.
type KollectClusterInventoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectClusterInventory `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectClusterInventory{}, &KollectClusterInventoryList{})
}
