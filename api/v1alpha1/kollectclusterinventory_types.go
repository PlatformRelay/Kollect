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
// (ADR-0703). Reconciled by KollectClusterInventoryReconciler.
type KollectClusterInventorySpec struct {
	// profileRef names a KollectClusterProfile stub for shared extraction schema (optional).
	// +optional
	ProfileRef string `json:"profileRef,omitempty"`

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

	// snapshotSinkRefs lists KollectSnapshotSink or KollectClusterSnapshotSink names (ADR-0414).
	// +optional
	SnapshotSinkRefs InventorySinkRefList `json:"snapshotSinkRefs,omitempty"`

	// databaseSinkRefs lists KollectDatabaseSink or KollectClusterDatabaseSink names (ADR-0414).
	// +optional
	DatabaseSinkRefs InventorySinkRefList `json:"databaseSinkRefs,omitempty"`

	// eventSinkRefs lists KollectEventSink or KollectClusterEventSink names (ADR-0414).
	// +optional
	EventSinkRefs InventorySinkRefList `json:"eventSinkRefs,omitempty"`

	// sinkNamespace is the namespace where namespaced family sinks are resolved.
	// +kubebuilder:default="kollect-system"
	// +optional
	SinkNamespace string `json:"sinkNamespace,omitempty"`

	// exportMinInterval is the minimum time between identical exports for this inventory.
	// Material changes (payload checksum or spec generation) bypass the interval.
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
