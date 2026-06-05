// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectClusterInventorySpec defines platform-wide rollup across KollectClusterTarget objects
// (ADR-0032). No collection controller is registered in Phase 1 — API + webhook + samples only.
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

	// sinkRefs lists KollectSink names resolved in sinkNamespace.
	// Namespaced sinks in the export namespace are the MVP path; KollectClusterSink is reserved.
	// +optional
	SinkRefs []string `json:"sinkRefs,omitempty"`

	// sinkNamespace is the namespace where namespaced KollectSink objects are resolved.
	// +kubebuilder:default="kollect-system"
	// +optional
	SinkNamespace string `json:"sinkNamespace,omitempty"`

	// exportMinInterval is the minimum time between identical exports for this inventory.
	// Material changes (payload checksum or spec generation) bypass the interval.
	// +kubebuilder:default="30s"
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`

	// suspend pauses reconciliation when set to true (reserved for future controller).
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// KollectClusterInventoryStatus is reserved for a future cluster-scoped rollup controller.
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

	// lastExportTime is the timestamp of the last successful export.
	// +optional
	LastExportTime *metav1.Time `json:"lastExportTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kcinv
// +kubebuilder:storageversion

// KollectClusterInventory rolls up cluster targets for platform operators.
type KollectClusterInventory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KollectClusterInventorySpec   `json:"spec"`
	Status KollectClusterInventoryStatus `json:"status,omitzero"`
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
