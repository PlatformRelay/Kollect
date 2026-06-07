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
	DatabaseSinkTypeMongoDB  = "mongodb"
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

	// serialization configures the cross-cutting on-wire format and schema contract (ADR-0416 §4).
	// json is the zero-config default; the backend capability matrix gates which formats are honored.
	// +optional
	Serialization *SerializationSpec `json:"serialization,omitempty"`

	// layout configures document shape and folder layout for snapshot Git/GitLab sinks (ADR-0419).
	// The entire block is optional; omitting it yields a single readable inventory document.
	// +optional
	Layout *LayoutSpec `json:"layout,omitempty"`

	// provisioning configures destination resource ownership (ADR-0416 §5).
	// mode ensure (default) creates resources if missing; existing never creates and preflights existence.
	// +optional
	Provisioning *ProvisioningSpec `json:"provisioning,omitempty"`

	// options carries non-secret, backend-specific pass-through settings (ADR-0416 §4, Option 2).
	// Secret-like keys are rejected by the webhook; supply credentials via secretRef only.
	// +optional
	Options map[string]string `json:"options,omitempty"`
}

// SerializationSpec is the cross-cutting serialization and schema block shared by all sink
// families (ADR-0416 §4). Honored fields depend on the backend capability matrix.
type SerializationSpec struct {
	// format selects the on-wire serialization (default json; yaml default for git/gitlab).
	// +kubebuilder:validation:Enum=json;yaml;parquet;csv;ndjson
	// +optional
	Format string `json:"format,omitempty"`

	// compression selects payload compression where the backend supports it (default none).
	// +kubebuilder:validation:Enum=none;gzip;snappy;zstd
	// +optional
	Compression string `json:"compression,omitempty"`
}

// ProvisioningSpec is the cross-cutting resource-ownership block shared by all sink families
// (ADR-0416 §5). It generalizes "who creates and owns the destination topic/table/bucket".
type ProvisioningSpec struct {
	// mode selects ensure (create-if-missing, default) or existing (never create; preflight verifies).
	// +kubebuilder:validation:Enum=ensure;existing
	// +optional
	Mode string `json:"mode,omitempty"`

	// naming optionally templates the destination resource name using the shared placeholder grammar.
	// +optional
	Naming *ProvisioningNamingSpec `json:"naming,omitempty"`
}

// ProvisioningNamingSpec templates the destination resource name (ADR-0416 §5).
type ProvisioningNamingSpec struct {
	// template is the destination name template; placeholders match pathTemplate
	// ({cluster}, {namespace}, {name}, {generation}).
	// +optional
	Template string `json:"template,omitempty"`
}

// FamilySinkStatus is the shared status shape for family sink CRDs.
type FamilySinkStatus struct {
	// conditions represent the current state of the sink resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// preview is a read-only, side-effect-free preview of export implications, populated when the
	// kollect.dev/preview annotation is set (ADR-0416 §8).
	// +optional
	Preview *SinkPreviewStatus `json:"preview,omitempty"`
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
