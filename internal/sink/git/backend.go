// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// Backend exports inventory payloads to a git remote (Phase 1 stub).
type Backend struct {
	cfg Config
}

// NewBackend constructs a git sink backend from spec and optional resolved CA PEM.
func NewBackend(spec kollectdevv1alpha1.KollectSinkSpec, caPEM []byte) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, caPEM)
	if err != nil {
		return nil, err
	}

	return &Backend{cfg: cfg}, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return "git"
}

// Config exposes the resolved configuration (for connection tests).
func (b *Backend) Config() Config {
	return b.cfg
}

// Export is a Phase 1 placeholder for Git push wiring.
func (b *Backend) Export(ctx context.Context, payload []byte) error {
	_ = ctx
	_ = payload

	return nil
}
