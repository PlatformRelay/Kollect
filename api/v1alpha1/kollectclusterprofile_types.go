// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectClusterProfileSpec defines the desired state of KollectClusterProfile.
// Same shape as KollectProfileSpec — cluster-scoped platform extraction schema (ADR-0204).
type KollectClusterProfileSpec struct {
	// targetGVK selects the Kubernetes resource kind this profile applies to.
	// +required
	TargetGVK GroupVersionKind `json:"targetGVK"`

	// attributes lists the values to extract from each matching resource.
	// Required when export.mode is Attributes (default); optional when export.mode is Resource.
	// +listType=map
	// +listMapKey=name
	// +optional
	Attributes []AttributeSpec `json:"attributes,omitempty"`

	// export optionally enables full-resource export with path pruning (ADR-0306).
	// When omitted, the profile behaves as export.mode: Attributes.
	// +optional
	Export *ExportSpec `json:"export,omitempty"`

	// metrics lists kube-state-metrics-style Prometheus series emitted on operator /metrics.
	// +listType=map
	// +listMapKey=name
	// +optional
	Metrics []MetricSpec `json:"metrics,omitempty"`
}

// KollectClusterProfileStatus defines the observed state of KollectClusterProfile.
type KollectClusterProfileStatus struct {
	// conditions represent the current state of the KollectClusterProfile resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kcprof
// +kubebuilder:storageversion

// KollectClusterProfile is the Schema for platform-wide shared extraction schemas.
type KollectClusterProfile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectClusterProfile
	// +required
	Spec KollectClusterProfileSpec `json:"spec"`

	// status defines the observed state of KollectClusterProfile
	// +optional
	Status KollectClusterProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectClusterProfileList contains a list of KollectClusterProfile.
type KollectClusterProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectClusterProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectClusterProfile{}, &KollectClusterProfileList{})
}
