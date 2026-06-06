// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sink family identifiers for inventory refs and resolver (ADR-0414).
const (
	SinkFamilySnapshot = "snapshot"
	SinkFamilyDatabase = "database"
	SinkFamilyEvent    = "event"
)

// Snapshot sink type values (KollectSnapshotSink.spec.type).
const (
	SnapshotSinkTypeGit       = "git"
	SnapshotSinkTypeGitLab    = "gitlab"
	SnapshotSinkTypeS3        = "s3"
	SnapshotSinkTypeGCS       = "gcs"
	SnapshotSinkTypeAzureBlob = "azureblob"
	SnapshotSinkTypeHTTP      = "http"
)

// Database sink type values (KollectDatabaseSink.spec.type).
const (
	DatabaseSinkTypePostgres = "postgres"
	DatabaseSinkTypeBigQuery = "bigquery"
)

// Event sink type values (KollectEventSink.spec.type).
const (
	EventSinkTypeNats  = "nats"
	EventSinkTypeKafka = "kafka"
)

// SinkCommonFields holds configuration shared across all sink family CRDs (ADR-0414).
type SinkCommonFields struct {
	// endpoint is the backend-specific destination (URL, bucket, and so on).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// secretRef references a Secret holding credentials for the sink.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// tls configures TLS verification for HTTPS git and similar endpoints.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// connectionTest enables connectivity checks on create/update (default true).
	// +optional
	ConnectionTest *bool `json:"connectionTest,omitempty"`

	// cluster labels exported inventory in multi-cluster installs.
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// pathTemplate selects the Git/object-store export path layout (ADR-0407).
	// +optional
	PathTemplate string `json:"pathTemplate,omitempty"`

	// exportMinInterval is the default minimum time between identical exports when an inventory
	// ref omits a per-ref override.
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`
}

// FamilySinkStatus is the shared status shape for family sink CRDs.
type FamilySinkStatus struct {
	// conditions represent the current state of the sink resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// HTTPSinkSpec configures HTTP/webhook snapshot export (stub — ADR-0414).
type HTTPSinkSpec struct {
	// method is the HTTP verb for export requests (default POST).
	// +optional
	Method string `json:"method,omitempty"`
}

// BigQuerySpec configures BigQuery relational export (stub — ADR-0414).
type BigQuerySpec struct {
	// dataset is the BigQuery dataset id.
	// +optional
	Dataset string `json:"dataset,omitempty"`

	// table is the destination table name.
	// +optional
	Table string `json:"table,omitempty"`
}

// ConnectionTestEnabledCommon reports whether automatic connectivity probes should run.
func ConnectionTestEnabledCommon(fields *SinkCommonFields) bool {
	if fields == nil || fields.ConnectionTest == nil {
		return true
	}
	return *fields.ConnectionTest
}
