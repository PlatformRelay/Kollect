// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectClusterTargetSpec defines platform-wide collection across namespaces (ADR-0201).
// No collection controller is registered in Phase 1 — API + webhook + samples only.
type KollectClusterTargetSpec struct {
	// profileRef points at a namespaced KollectProfile by name and namespace (ADR-0208).
	// namespace is required at admission — there is no implicit platform-namespace fallback.
	// +required
	ProfileRef NamespacedObjectReference `json:"profileRef"`

	// namespaceSelector restricts collection to namespaces matching the selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	CollectionFilterSpec `json:",inline"`

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

	// collectedCount is the number of resources this cluster target was collecting when the
	// controller last refreshed the count. Semantics are identical to KollectTarget's
	// collectedCount (design D3 — one contract, twice): null means never computed, zero is a
	// measured zero, and a Degraded target keeps its last known count.
	// +optional
	CollectedCount *int64 `json:"collectedCount,omitempty"`

	// collectedCountUpdatedAt is when collectedCount last *changed* — not when it was last
	// checked. It moves only when the number moves, so it pairs with the conditions to tell
	// a steady count from a stale one.
	// +optional
	CollectedCountUpdatedAt *metav1.Time `json:"collectedCountUpdatedAt,omitempty"`

	CollectionFilterStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kctgt
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Collected",type=integer,JSONPath=`.status.collectedCount`
// +kubebuilder:printcolumn:name="Updated",type=date,JSONPath=`.status.collectedCountUpdatedAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
