// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KollectEventSinkSpec defines an event-stream export sink (ADR-0414).
type KollectEventSinkSpec struct {
	// type selects the event backend implementation.
	// +kubebuilder:validation:Enum=nats;kafka
	// +required
	Type string `json:"type"`

	SinkCommonFields `json:",inline"`

	// nats configures NATS JetStream inventory change events.
	// +optional
	Nats *NatsSpec `json:"nats,omitempty"`

	// kafka configures Kafka inventory change events.
	// +optional
	Kafka *KafkaSpec `json:"kafka,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=kevt

// KollectEventSink is the Schema for event export sinks.
type KollectEventSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KollectEventSinkSpec `json:"spec"`
	Status            FamilySinkStatus     `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectEventSinkList contains a list of KollectEventSink.
type KollectEventSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KollectEventSink `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kcevt

// KollectClusterEventSink is a cluster-scoped event export sink.
type KollectClusterEventSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KollectEventSinkSpec `json:"spec"`
	Status            FamilySinkStatus     `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectClusterEventSinkList contains a list of KollectClusterEventSink.
type KollectClusterEventSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KollectClusterEventSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&KollectEventSink{}, &KollectEventSinkList{},
		&KollectClusterEventSink{}, &KollectClusterEventSinkList{},
	)
}
