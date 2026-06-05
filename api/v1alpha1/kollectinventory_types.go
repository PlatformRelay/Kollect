// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KollectInventorySpec defines the desired state of KollectInventory.
type KollectInventorySpec struct {
	// sinkRefs lists KollectSink names in the same namespace as this Inventory.
	// Each entry may be a plain name or an object with an optional exportMinInterval override.
	// +optional
	SinkRefs InventorySinkRefList `json:"sinkRefs,omitempty"`

	// exportMinInterval is the minimum time between identical exports for this inventory.
	// Material changes (payload checksum or spec generation) bypass the interval.
	// +kubebuilder:default="30s"
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`

	// maxExportBytes caps the marshalled namespace payload for export and HTTP (optional).
	// Webhook rejects values above the operator global cap (ADR-0103).
	// +optional
	MaxExportBytes *int64 `json:"maxExportBytes,omitempty"`

	// suspend pauses reconciliation of this inventory when set to true.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// httpEndpoint exposes a read-only inventory summary over HTTP when enabled.
	// +optional
	HTTPEndpoint *HTTPEndpointConfig `json:"httpEndpoint,omitempty"`
}

// HTTPEndpointConfig toggles the operator inventory HTTP server.
type HTTPEndpointConfig struct {
	// enabled turns on GET /v1alpha1/inventory (aggregated summary JSON).
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// port is the listen port for the inventory HTTP server (default 8082).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

// KollectInventoryStatus defines the observed state of KollectInventory.
type KollectInventoryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// See Kubernetes API conventions for typical status properties.

	// conditions represent the current state of the KollectInventory resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// itemCount is the number of inventory items collected in the last run.
	// +optional
	ItemCount int `json:"itemCount,omitempty"`

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
// +kubebuilder:resource:shortName=kinv

// KollectInventory is the Schema for the kollectinventories API
type KollectInventory struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectInventory
	// +required
	Spec KollectInventorySpec `json:"spec"`

	// status defines the observed state of KollectInventory
	// +optional
	Status KollectInventoryStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectInventoryList contains a list of KollectInventory
type KollectInventoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectInventory `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectInventory{}, &KollectInventoryList{})
}
