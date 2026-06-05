// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectRemoteClusterSpec registers a spoke cluster on the hub (ADR-0028).
type KollectRemoteClusterSpec struct {
	// clusterName is the stable DNS-1123 identifier for the spoke cluster.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	ClusterName string `json:"clusterName"`

	// credentialsSecretRef points to an Istio-style remote kubeconfig secret for optional hub pull.
	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`

	// apiServerURL is the spoke Kubernetes API server URL (optional for push-only spokes).
	// +optional
	APIServerURL string `json:"apiServerURL,omitempty"`

	// trustBundle is a PEM-encoded CA bundle for spoke API TLS or future mTLS (optional).
	// +optional
	TrustBundle string `json:"trustBundle,omitempty"`
}

// KollectRemoteClusterStatus reports minimal hub-side observation (full reconciler deferred).
type KollectRemoteClusterStatus struct {
	// conditions represent the current state of the KollectRemoteCluster resource.
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
// +kubebuilder:resource:shortName=kremote
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterName`
// +kubebuilder:printcolumn:name="Connected",type=string,JSONPath=`.status.conditions[?(@.type=="Connected")].status`

// KollectRemoteCluster declares a registered spoke cluster on the inventory hub.
type KollectRemoteCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KollectRemoteClusterSpec   `json:"spec"`
	Status KollectRemoteClusterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectRemoteClusterList contains a list of KollectRemoteCluster.
type KollectRemoteClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectRemoteCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectRemoteCluster{}, &KollectRemoteClusterList{})
}
