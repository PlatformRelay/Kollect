// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package local implements a filesystem sink backend for the pipeline CLI mode (ADR-0801):
// it writes export payloads directly to a local output directory instead of a remote
// git/object-store/database destination. CI jobs handle version control themselves.
package local

import (
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TypeName is the sink type string that selects this backend ("type: local").
const TypeName = "local"

// Config holds the resolved settings for the local filesystem backend.
type Config struct {
	OutputDir string
}

// ConfigFromSpec validates spec and extracts the local backend configuration.
// spec.Endpoint holds the output directory path.
func ConfigFromSpec(spec kollectdevv1alpha1.KollectSinkSpec) (Config, error) {
	if spec.Type != TypeName {
		return Config{}, fmt.Errorf("local sink: expected type %q, got %q", TypeName, spec.Type)
	}
	if spec.Endpoint == "" {
		return Config{}, fmt.Errorf("local sink: spec.endpoint (output directory) is required")
	}

	return Config{OutputDir: spec.Endpoint}, nil
}
