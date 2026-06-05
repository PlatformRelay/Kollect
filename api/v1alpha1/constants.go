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
)
