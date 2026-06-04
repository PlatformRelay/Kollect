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
	// sinkRefs lists the names of KollectSink resources to export the inventory to.
	// +optional
	SinkRefs []string `json:"sinkRefs,omitempty"`

	// suspend pauses reconciliation of this inventory when set to true.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
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

	// lastExportTime is the timestamp of the last successful export.
	// +optional
	LastExportTime *metav1.Time `json:"lastExportTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kinv

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
