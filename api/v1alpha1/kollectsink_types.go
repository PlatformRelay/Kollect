// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KollectSinkSpec defines the desired state of KollectSink.
type KollectSinkSpec struct {
	// type selects the sink backend implementation.
	// +kubebuilder:validation:Enum=git;gitlab;s3;gcs;prometheus
	// +required
	Type string `json:"type"`

	// endpoint is the backend-specific destination (URL, bucket, and so on).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// secretRef references a Secret holding credentials for the sink.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// tls configures TLS verification for HTTPS git and similar endpoints.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// connectionTest requests a connectivity check on create/update when true.
	// The annotation kollect.dev/test-connection=true has the same effect.
	// +optional
	ConnectionTest bool `json:"connectionTest,omitempty"`
}

// TLSSpec configures custom CA trust for sink endpoints.
type TLSSpec struct {
	// insecureSkipVerify disables server certificate verification (not recommended).
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// caBundle is an inline PEM-encoded CA certificate bundle.
	// Prefer caSecretRef for production; do not set both caBundle and caSecretRef.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// caSecretRef references a Secret containing a PEM CA bundle (key tls.crt or ca.crt).
	// +optional
	CASecretRef *SecretReference `json:"caSecretRef,omitempty"`
}

// SecretReference points to a Secret by name and optional namespace.
type SecretReference struct {
	// name is the name of the referenced Secret.
	// +required
	Name string `json:"name"`

	// namespace is the namespace of the referenced Secret.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// KollectSinkStatus defines the observed state of KollectSink.
type KollectSinkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// See Kubernetes API conventions for typical status properties.

	// conditions represent the current state of the KollectSink resource.
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
// +kubebuilder:resource:scope=Cluster,shortName=ksink

// KollectSink is the Schema for the kollectsinks API
type KollectSink struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KollectSink
	// +required
	Spec KollectSinkSpec `json:"spec"`

	// status defines the observed state of KollectSink
	// +optional
	Status KollectSinkStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectSinkList contains a list of KollectSink
type KollectSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectSink{}, &KollectSinkList{})
}
