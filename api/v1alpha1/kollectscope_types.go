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

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=kscope

// KollectScope is a namespaced governance boundary for targets, inventories, and sinks.
// Static config only — no controller or status subresource (ADR-0015).
type KollectScope struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec KollectScopeSpec `json:"spec"`
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
