// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Condition and annotation keys used across reconcilers and sinks.
const (
	ConditionConnectionVerified  = "ConnectionVerified"
	ConditionTLSInsecure         = "TLSInsecure"
	ConditionConnected           = "Connected"
	ConditionCredentialsVerified = "CredentialsVerified"
	ConditionSinkReachable       = "SinkReachable"
	ConditionSynced              = "Synced"
	ConditionExportSucceeded     = "ExportSucceeded"
	ConditionReady               = "Ready"
	ConditionDegraded            = "Degraded"

	ReasonExportTerminal = "ExportTerminal"

	AnnotationTestConnection = "kollect.dev/test-connection"

	// AnnotationPreview opts a sink into status.preview rendering of its export
	// implications without side effects (ADR-0416 §8).
	AnnotationPreview = "kollect.dev/preview"

	// Multi-cluster registration (Istio remote-secret parallel — ADR-0503).
	LabelMultiCluster        = "kollect.dev/multiCluster"
	AnnotationClusterName    = "kollect.dev/cluster"
	AnnotationSpokePrincipal = "kollect.dev/spokePrincipal"
	HeaderClusterID          = "X-Kollect-Cluster-Id"
	//nolint:gosec // G101: Istio-style remote secret name prefix, not a credential
	RemoteSecretNamePrefix = "kollect-remote-secret-"

	// Watch opt-in/opt-out labels and annotations (ADR-0205).
	// LabelWatch applies to namespaces and namespaced resources.
	LabelWatch = "kollect.dev/watch"
	// AnnotationNamespaceWatch applies to Namespace objects; affects all resources in the namespace
	// unless overridden by LabelWatch on the resource.
	AnnotationNamespaceWatch = "kollect.dev/namespace-watch"

	WatchValueEnabled  = "enabled"
	WatchValueDisabled = "disabled"

	WatchModeAll   = "All"
	WatchModeOptIn = "OptIn"
)

// Cross-cutting serialization formats for the serialization block (ADR-0416 §4).
// json is the zero-config default; backend capability gates which others are honored.
// yaml is the Git/GitLab default for human-readable snapshots (ADR-0419).
const (
	SerializationFormatJSON    = "json"
	SerializationFormatYAML    = "yaml"
	SerializationFormatParquet = "parquet"
	SerializationFormatCSV     = "csv"
	SerializationFormatNDJSON  = "ndjson"
)

// Provisioning ownership modes for the provisioning block (ADR-0416 §5).
//   - ensure (default): create destination resources if missing; never destructive.
//   - existing: never issue create/admin calls; preflight verifies the resource exists.
const (
	ProvisioningModeEnsure   = "ensure"
	ProvisioningModeExisting = "existing"
)
