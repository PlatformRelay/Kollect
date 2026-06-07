// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package export defines the versioned inventory export data contract (ADR-0405).
package export

import "fmt"

const (
	// WireAPIVersion is the export envelope apiVersion field (ADR-0405).
	WireAPIVersion = "kollect.dev/v1alpha1"

	// SchemaVersion is the export envelope contract version (ADR-0405).
	// Initially aligned with WireAPIVersion; bumped only on breaking contract changes.
	SchemaVersion = WireAPIVersion
)

var supportedSchemaVersions = map[string]struct{}{
	SchemaVersion: {},
}

// NormalizeSchemaVersion returns v or the current default when v is empty.
func NormalizeSchemaVersion(v string) string {
	if v == "" {
		return SchemaVersion
	}

	return v
}

// ValidateSchemaVersion rejects unsupported export contract versions.
func ValidateSchemaVersion(v string) error {
	v = NormalizeSchemaVersion(v)
	if _, ok := supportedSchemaVersions[v]; !ok {
		return fmt.Errorf("unsupported schemaVersion %q", v)
	}

	return nil
}
