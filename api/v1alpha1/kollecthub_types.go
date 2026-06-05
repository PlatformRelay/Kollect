// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HubTransportSpec selects the lean queue backend for hub fan-in (ADR-0502).
type HubTransportSpec struct {
	// type selects the transport implementation.
	// +kubebuilder:validation:Enum=inprocess;redis;nats;kafka
	// +required
	Type string `json:"type"`

	// redis configures a Redis Streams backend when type is redis.
	// +optional
	Redis *RedisTransportSpec `json:"redis,omitempty"`
}

// RedisTransportSpec holds Redis connection settings.
type RedisTransportSpec struct {
	// url is a Redis connection URL (redis:// or rediss://).
	// +required
	URL string `json:"url"`
}

// RemoteClusterRef references a registered KollectRemoteCluster on the hub (ADR-0503).
type RemoteClusterRef struct {
	// name of the KollectRemoteCluster resource.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// namespace of the KollectRemoteCluster; defaults to kollect-system when empty.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// Deprecated: KollectHub is not a product surface — use Helm mode: hub|spoke (ADR-0703).
// The CRD remains as a reserved stub; no controller is registered.

// KollectHubSpec defines the desired state of KollectHub.
type KollectHubSpec struct {
	// replicas is the hub consumer Deployment replica count.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// transport configures the spoke-to-hub queue backend.
	// +required
	Transport HubTransportSpec `json:"transport"`

	// remoteClusters lists spoke registrations wired into this hub consumer (ADR-0503).
	// +optional
	RemoteClusters []RemoteClusterRef `json:"remoteClusters,omitempty"`
}

// KollectHubStatus defines the observed state of KollectHub.
type KollectHubStatus struct {
	// conditions represent the current state of the KollectHub resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// registeredRemoteClusters is the count of resolved remoteClusters refs.
	// +optional
	RegisteredRemoteClusters int32 `json:"registeredRemoteClusters,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=khub

// KollectHub declares a multi-cluster hub Deployment and transport queue.
type KollectHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KollectHubSpec   `json:"spec"`
	Status KollectHubStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KollectHubList contains a list of KollectHub.
type KollectHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectHub `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectHub{}, &KollectHubList{})
}
