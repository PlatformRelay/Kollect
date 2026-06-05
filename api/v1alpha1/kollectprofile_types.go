// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KollectProfileSpec defines the desired state of KollectProfile.
type KollectProfileSpec struct {
	// targetGVK selects the Kubernetes resource kind this profile applies to.
	// +required
	TargetGVK GroupVersionKind `json:"targetGVK"`

	// attributes lists the values to extract from each matching resource.
	// +listType=map
	// +listMapKey=name
	// +optional
	Attributes []AttributeSpec `json:"attributes,omitempty"`
}

// GroupVersionKind identifies the API group, version, and kind of a target resource.
type GroupVersionKind struct {
	// group is the API group of the target resource (empty for the core group).
	// +optional
	Group string `json:"group,omitempty"`

	// version is the API version of the target resource.
	// +required
	Version string `json:"version"`

	// kind is the API kind of the target resource.
	// +required
	Kind string `json:"kind"`
}

// AttributeSpec describes a single value extracted from a target resource.
type AttributeSpec struct {
	// name is the unique key under which the extracted value is stored.
	// +required
	Name string `json:"name"`

	// path is the JSONPath or CEL expression used to extract the value.
	// +required
	Path string `json:"path"`

	// type is the expected value type (for example string, int, or bool).
	// +optional
	Type string `json:"type,omitempty"`

	// optional marks the attribute as non-fatal when extraction yields no value.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// KollectProfileStatus defines the observed state of KollectProfile.
type KollectProfileStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// See Kubernetes API conventions for typical status properties.

	// conditions represent the current state of the KollectProfile resource.
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=kprof

// KollectProfile is the Schema for the kollectprofiles API
type KollectProfile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectProfile
	// +required
	Spec KollectProfileSpec `json:"spec"`

	// status defines the observed state of KollectProfile
	// +optional
	Status KollectProfileStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectProfileList contains a list of KollectProfile
type KollectProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectProfile{}, &KollectProfileList{})
}
