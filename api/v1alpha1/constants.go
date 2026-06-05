// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Condition and annotation keys used across reconcilers and sinks.
const (
	ConditionConnectionVerified = "ConnectionVerified"
	ConditionTLSInsecure        = "TLSInsecure"

	AnnotationTestConnection = "kollect.dev/test-connection"
)
