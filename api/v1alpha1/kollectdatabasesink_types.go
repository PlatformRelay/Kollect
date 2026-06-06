// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KollectDatabaseSinkSpec defines a relational database export sink (ADR-0414).
type KollectDatabaseSinkSpec struct {
	// type selects the database backend implementation.
	// +kubebuilder:validation:Enum=postgres;bigquery
	// +required
	Type string `json:"type"`

	SinkCommonFields `json:",inline"`

	// postgres configures PostgreSQL upsert export.
	// +optional
	Postgres *PostgresSpec `json:"postgres,omitempty"`

	// bigquery configures BigQuery export (stub).
	// +optional
	BigQuery *BigQuerySpec `json:"bigquery,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=kdb

// KollectDatabaseSink is the Schema for relational export sinks.
type KollectDatabaseSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KollectDatabaseSinkSpec `json:"spec"`
	Status            FamilySinkStatus        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectDatabaseSinkList contains a list of KollectDatabaseSink.
type KollectDatabaseSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KollectDatabaseSink `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kcdb

// KollectClusterDatabaseSink is a cluster-scoped relational export sink.
type KollectClusterDatabaseSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KollectDatabaseSinkSpec `json:"spec"`
	Status            FamilySinkStatus        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KollectClusterDatabaseSinkList contains a list of KollectClusterDatabaseSink.
type KollectClusterDatabaseSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KollectClusterDatabaseSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&KollectDatabaseSink{}, &KollectDatabaseSinkList{},
		&KollectClusterDatabaseSink{}, &KollectClusterDatabaseSinkList{},
	)
}
