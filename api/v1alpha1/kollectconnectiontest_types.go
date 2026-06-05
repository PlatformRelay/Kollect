// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KollectConnectionTestSpec defines a one-shot connectivity probe (ADR-0032).
type KollectConnectionTestSpec struct {
	// sinkRef is the name of a KollectSink in the same namespace.
	// +required
	SinkRef string `json:"sinkRef"`

	// profileRef optionally names a KollectProfile for future composite probes.
	// +optional
	ProfileRef string `json:"profileRef,omitempty"`

	// ownerSink sets an ownerReference to the sink when true (default).
	// +optional
	OwnerSink *bool `json:"ownerSink,omitempty"`

	// ttlSecondsAfterFinished deletes the CR after status.completed plus this TTL.
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=0
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// KollectConnectionTestStatus reports probe outcome.
type KollectConnectionTestStatus struct {
	// conditions represent the current state of the connection test.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// latencyMs is the duration of the last probe in milliseconds.
	// +optional
	LatencyMs int64 `json:"latencyMs,omitempty"`

	// completed is true after the probe has finished (success or failure).
	// +optional
	Completed bool `json:"completed,omitempty"`

	// completedAt is set when completed becomes true (used for TTL cleanup).
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kconntest
// +kubebuilder:printcolumn:name="Sink",type=string,JSONPath=`.spec.sinkRef`
//nolint:lll // kubebuilder printcolumn marker must stay on one line
// +kubebuilder:printcolumn:name="Verified",type=string,JSONPath=`.status.conditions[?(@.type=="ConnectionVerified")].status`

// KollectConnectionTest triggers an audited one-shot sink connectivity probe.
type KollectConnectionTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KollectConnectionTestSpec   `json:"spec"`
	Status KollectConnectionTestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectConnectionTestList contains a list of KollectConnectionTest.
type KollectConnectionTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KollectConnectionTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KollectConnectionTest{}, &KollectConnectionTestList{})
}
