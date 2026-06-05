// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectClusterTargetSpec defines platform-wide collection across namespaces (ADR-0032).
// No collection controller is registered in Phase 1 — API + webhook + samples only.
type KollectClusterTargetSpec struct {
	// profileRef names a KollectClusterProfile or a platform-namespace KollectProfile stub.
	// +required
	ProfileRef string `json:"profileRef"`

	// namespaceSelector restricts collection to namespaces matching the selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// suspend pauses reconciliation when set to true (reserved for future controller).
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// KollectClusterTargetStatus is reserved for a future cluster-scoped collection controller.
type KollectClusterTargetStatus struct {
	// conditions represent the current state of the KollectClusterTarget resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed by a controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kctgt
// +kubebuilder:storageversion

// KollectClusterTarget selects resources cluster-wide for platform operators.
type KollectClusterTarget struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectClusterTarget
	// +required
	Spec KollectClusterTargetSpec `json:"spec"`

	// status defines the observed state of KollectClusterTarget
	// +optional
	Status KollectClusterTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectClusterTargetList contains a list of KollectClusterTarget.
type KollectClusterTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectClusterTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectClusterTarget{}, &KollectClusterTargetList{})
}
