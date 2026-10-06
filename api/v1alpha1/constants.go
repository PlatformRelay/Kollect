// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Condition and annotation keys used across reconcilers and sinks.
const (
	ConditionConnectionVerified = "ConnectionVerified"
	ConditionTLSInsecure        = "TLSInsecure"
	ConditionSinkReachable      = "SinkReachable"
	ConditionSynced             = "Synced"
	ConditionExportSucceeded    = "ExportSucceeded"
	ConditionReady              = "Ready"
	ConditionDegraded           = "Degraded"

	ReasonExportTerminal = "ExportTerminal"

	AnnotationTestConnection = "kollect.dev/test-connection"

	// AnnotationForceCleanup is a one-field escape hatch: set to "true" on a
	// KollectInventory / KollectClusterInventory whose sink cleanup failed
	// terminally (e.g. a revoked git token at delete time) to drop the cleanup
	// finalizer without contacting the backend, letting deletion complete (K-30).
	// Honored only while the object carries a deletionTimestamp — it can never
	// skip cleanup of a live object.
	AnnotationForceCleanup = "kollect.dev/force-cleanup"

	// ForceCleanupTrue is the only accepted value of AnnotationForceCleanup.
	ForceCleanupTrue = "true"

	// AnnotationPreview opts a sink into status.preview rendering of its export
	// implications without side effects (ADR-0416 §8).
	AnnotationPreview = "kollect.dev/preview"

	// AnnotationRequestedAt is the manual re-export trigger on the two
	// inventory kinds whose reconcilers own an export debounce
	// (KollectInventory, KollectClusterInventory). The reconcilers treat any
	// change of its value — appearing on an object that had none, removal
	// after being set, or a different value — as a one-export invalidation of
	// the per-sink export debounce: the next reconcile exports to every sink
	// binding that would otherwise be debounced, then the steady-state
	// debounce resumes unchanged. The value is not parsed (any non-empty
	// string works as a trigger, RFC3339 is the documented convention only)
	// and absence counts as a value. Kinds without an export debounce honour
	// nothing here.
	AnnotationRequestedAt = "kollect.dev/requestedAt"

	// AnnotationCollectedGeneration records the source object's
	// metadata.generation on the embedded copy a Resource-mode profile exports
	// (Export.mode: Resource). It is stamped after profile pruning and
	// scrubbing, so prune paths and scrub rules cannot remove or redact it,
	// and it is written whenever a metadata map survives the profile's include
	// section (generation 0 included); no metadata map means no stamp — the
	// default SpecAndStatus include drops metadata, so profiles that want the
	// stamp set include: All or MetadataOnly. Attributes-mode exports carry no
	// embedded copy and no stamp.
	AnnotationCollectedGeneration = "kollect.dev/collectedGeneration"

	// Multi-cluster registration (Istio remote-secret parallel).
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

// Snapshot sink deletion policies (KollectSnapshotSink.spec.deletionPolicy, ADR-0421).
//   - Retain (default): inventory deletion leaves exported objects in place.
//   - Delete: inventory deletion retracts the objects the sink exported for it.
const (
	DeletionPolicyRetain = "Retain"
	DeletionPolicyDelete = "Delete"
)

// Provisioning ownership modes for the provisioning block (ADR-0416 §5).
//   - ensure (default): create destination resources if missing; never destructive.
//   - existing: never issue create/admin calls; preflight verifies the resource exists.
const (
	ProvisioningModeEnsure   = "ensure"
	ProvisioningModeExisting = "existing"
)
