// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"context"
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/s3"
)

// TypeName is the KollectSink.spec.type value for GCS sinks.
const TypeName = "gcs"

const typeName = TypeName

// Backend uploads inventory payloads via the GCS S3-compatible XML API.
type Backend struct {
	inner *s3.Backend
}

// NewBackend constructs a GCS sink using S3-compatible credentials and endpoint.
func NewBackend(spec kollectdevv1alpha1.KollectSinkSpec, creds map[string][]byte) (*Backend, error) {
	if spec.Type != typeName {
		return nil, fmt.Errorf("expected gcs sink, got %q", spec.Type)
	}

	clone := spec
	clone.Type = "s3"

	inner, err := s3.NewBackend(clone, creds)
	if err != nil {
		return nil, err
	}

	return &Backend{inner: inner}, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Capabilities reports whole-snapshot export (ADR-0401).
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.SnapshotStore()
}

// Export uploads payload at objectPath.
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	return b.inner.Export(ctx, payload, objectPath)
}
