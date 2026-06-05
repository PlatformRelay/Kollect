// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectScopeSpec defines a namespaced tenancy boundary for collection and sinks.
type KollectScopeSpec struct {
	// allowedGVKs restricts which target resource kinds may be collected in this scope.
	// +listType=atomic
	// +optional
	AllowedGVKs []GroupVersionKind `json:"allowedGVKs,omitempty"`

	// allowedNamespaces restricts which workload namespaces may be collected.
	// Empty means any namespace allowed by targets in the scope namespace.
	// +listType=set
	// +optional
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`

	// sinkRefs lists cluster-scoped KollectSink names permitted for export from this scope.
	// +listType=set
	// +optional
	SinkRefs []string `json:"sinkRefs,omitempty"`
}

// KollectScopeStatus defines the observed state of KollectScope.
type KollectScopeStatus struct {
	// conditions represent the current state of the KollectScope resource.
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
// +kubebuilder:resource:shortName=kscope

// KollectScope is a namespaced governance boundary for targets, inventories, and sinks.
type KollectScope struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KollectScopeSpec   `json:"spec"`
	Status KollectScopeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectScopeList contains a list of KollectScope.
type KollectScopeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectScope `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectScope{}, &KollectScopeList{})
}
