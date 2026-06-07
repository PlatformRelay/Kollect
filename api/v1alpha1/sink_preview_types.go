// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// SinkPreviewStatus is a read-only, side-effect-free preview of a sink's export implications,
// rendered against a synthetic sample when the kollect.dev/preview annotation is set (ADR-0416 §8).
type SinkPreviewStatus struct {
	// renderedAt is the time the preview was last computed.
	// +optional
	RenderedAt *metav1.Time `json:"renderedAt,omitempty"`

	// inputSampleSource names the sample input used to render the preview (e.g. synthetic).
	// +optional
	InputSampleSource string `json:"inputSampleSource,omitempty"`

	// provisioningMode is the effective provisioning mode (ensure or existing).
	// +optional
	ProvisioningMode string `json:"provisioningMode,omitempty"`

	// serializationFormat is the effective on-wire format (json, parquet, ...).
	// +optional
	SerializationFormat string `json:"serializationFormat,omitempty"`

	// objectPath is the rendered destination path/object key for snapshot sinks.
	// +optional
	ObjectPath string `json:"objectPath,omitempty"`

	// git previews the commit subject/body for git and gitlab snapshot sinks.
	// +optional
	Git *GitPreviewStatus `json:"git,omitempty"`

	// layout previews the resolved export layout for git and gitlab snapshot sinks (ADR-0419).
	// +optional
	Layout *LayoutPreviewStatus `json:"layout,omitempty"`

	// postgres previews the expected CREATE TABLE DDL for postgres sinks.
	// +optional
	Postgres *PostgresPreviewStatus `json:"postgres,omitempty"`

	// mongodb previews the expected identity index for mongodb sinks.
	// +optional
	MongoDB *MongoDBPreviewStatus `json:"mongodb,omitempty"`

	// kafka previews the destination topic for kafka sinks.
	// +optional
	Kafka *KafkaPreviewStatus `json:"kafka,omitempty"`

	// warnings lists non-blocking implications surfaced by the preview.
	// +optional
	// +listType=atomic
	Warnings []string `json:"warnings,omitempty"`
}

// GitPreviewStatus previews a rendered git commit for snapshot sinks.
type GitPreviewStatus struct {
	// sampleCommitSubject is the rendered commit subject line.
	// +optional
	SampleCommitSubject string `json:"sampleCommitSubject,omitempty"`

	// sampleCommitBody is the rendered commit body.
	// +optional
	SampleCommitBody string `json:"sampleCommitBody,omitempty"`
}

// LayoutPreviewStatus previews the resolved export layout for git/gitlab snapshot sinks (ADR-0419).
type LayoutPreviewStatus struct {
	// mode is the resolved layout mode (document, perResource, or split).
	// +optional
	Mode string `json:"mode,omitempty"`

	// content is the resolved per-file content shape (item, attributes, or manifest).
	// +optional
	Content string `json:"content,omitempty"`

	// prune reports whether stale files are removed on export (auto for perResource/split).
	// +optional
	Prune bool `json:"prune,omitempty"`

	// samplePaths lists example repo paths the layout would write for the synthetic sample.
	// +optional
	// +listType=atomic
	SamplePaths []string `json:"samplePaths,omitempty"`
}

// PostgresPreviewStatus previews the DDL a postgres sink would run in ensure mode.
type PostgresPreviewStatus struct {
	// expectedDDL is the CREATE TABLE statement used in provisioning.mode=ensure.
	// +optional
	ExpectedDDL string `json:"expectedDDL,omitempty"`
}

// MongoDBPreviewStatus previews the identity index a mongodb sink would create in ensure mode.
type MongoDBPreviewStatus struct {
	// expectedIndexKeys lists the fields of the unique identity index.
	// +optional
	// +listType=atomic
	ExpectedIndexKeys []string `json:"expectedIndexKeys,omitempty"`
}

// KafkaPreviewStatus previews the destination topic for a kafka sink.
type KafkaPreviewStatus struct {
	// topic is the destination topic events would be published to.
	// +optional
	Topic string `json:"topic,omitempty"`
}
