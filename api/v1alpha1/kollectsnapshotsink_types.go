// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KollectSnapshotSinkSpec defines a snapshot-store export sink (ADR-0414).
type KollectSnapshotSinkSpec struct {
	// type selects the snapshot backend implementation.
	// +kubebuilder:validation:Enum=git;gitlab;s3;gcs
	// +required
	Type string `json:"type"`

	SinkCommonFields `json:",inline"`

	// deletionPolicy decides what happens to the objects this sink exported for an
	// inventory when that inventory is deleted (ADR-0421). Retain (default) leaves
	// them in place; Delete retracts them (git/gitlab deletion commit, S3/GCS object
	// deletion). Retain keeps every exported object; on git/gitlab it commits only the removal of
	// the inventory's ownership record (ADR-0422) and never contacts S3/GCS. A failed cleanup keeps
	// the inventory's finalizer and retries until it is resolved.
	// +kubebuilder:validation:Enum=Retain;Delete
	// +kubebuilder:default=Retain
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`

	// git configures git sink settings when type is git.
	// +optional
	Git *GitSpec `json:"git,omitempty"`

	// gitlab configures GitLab-specific settings when type is gitlab.
	// +optional
	GitLab *GitLabSpec `json:"gitlab,omitempty"`

	// objectStore configures S3/GCS/Azure snapshot serialization.
	// +optional
	ObjectStore *ObjectStoreSpec `json:"objectStore,omitempty"`

	// http is a reserved snapshot type that is rejected by admission; it is not the optional Inventory HTTP read API.
	// No webhook export backend ships, so the block exists only to reserve the name.
	// +optional
	HTTP *HTTPSinkSpec `json:"http,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ksnap

// KollectSnapshotSink is the Schema for snapshot export sinks.
type KollectSnapshotSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KollectSnapshotSinkSpec `json:"spec"`
	Status            FamilySinkStatus        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectSnapshotSinkList contains a list of KollectSnapshotSink.
type KollectSnapshotSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KollectSnapshotSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectSnapshotSink{}, &KollectSnapshotSinkList{})
}
