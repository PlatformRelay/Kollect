// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Condition and annotation keys used across reconcilers and sinks.
const (
	ConditionConnectionVerified = "ConnectionVerified"
	ConditionTLSInsecure        = "TLSInsecure"
	ConditionConnected          = "Connected"

	AnnotationTestConnection = "kollect.dev/test-connection"

	// Multi-cluster registration (Istio remote-secret parallel — ADR-0028).
	LabelMultiCluster     = "kollect.dev/multiCluster"
	AnnotationClusterName = "kollect.dev/cluster"
	HeaderClusterID       = "X-Kollect-Cluster-Id"
	//nolint:gosec // G101: Istio-style remote secret name prefix, not a credential
	RemoteSecretNamePrefix = "kollect-remote-secret-"

	// Watch opt-in/opt-out labels and annotations (ADR-0029).
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
