// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectScopeSpec defines a namespaced tenancy boundary for collection and sinks.
type KollectScopeSpec struct {
	ScopeCeilingSpec `json:",inline"`

	// snapshotSinkRefs lists permitted KollectSnapshotSink names for this scope.
	// +listType=set
	// +optional
	SnapshotSinkRefs []string `json:"snapshotSinkRefs,omitempty"`

	// databaseSinkRefs lists permitted KollectDatabaseSink names for this scope.
	// +listType=set
	// +optional
	DatabaseSinkRefs []string `json:"databaseSinkRefs,omitempty"`

	// eventSinkRefs lists permitted KollectEventSink names for this scope.
	// +listType=set
	// +optional
	EventSinkRefs []string `json:"eventSinkRefs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=kscope

// KollectScope is a namespaced governance boundary for targets, inventories, and sinks.
// Static config only — no controller or status subresource (ADR-0202).
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
