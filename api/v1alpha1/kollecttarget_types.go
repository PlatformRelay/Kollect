// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KollectTargetSpec defines the desired state of KollectTarget.
type KollectTargetSpec struct {
	// profileRef is the name of the KollectProfile that defines what to collect.
	// +required
	ProfileRef string `json:"profileRef"`

	// namespaceSelector restricts collection to namespaces matching the selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// labelSelector restricts collection to resources matching the selector.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// names optionally restricts collection to resources with these names.
	// +optional
	Names []string `json:"names,omitempty"`

	// suspend pauses reconciliation of this target when set to true.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// KollectTargetStatus defines the observed state of KollectTarget.
type KollectTargetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// See Kubernetes API conventions for typical status properties.

	// conditions represent the current state of the KollectTarget resource.
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ktgt

// KollectTarget is the Schema for the kollecttargets API
type KollectTarget struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectTarget
	// +required
	Spec KollectTargetSpec `json:"spec"`

	// status defines the observed state of KollectTarget
	// +optional
	Status KollectTargetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectTargetList contains a list of KollectTarget
type KollectTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectTarget{}, &KollectTargetList{})
}
