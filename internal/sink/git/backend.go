// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
)

// Backend exports inventory payloads to a git remote.
type Backend struct {
	cfg  Config
	auth Auth
}

// NewBackend constructs a git sink backend from spec, optional resolved CA PEM, and credentials.
func NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	caPEM []byte,
	auth Auth,
	sshKnownHosts []byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, caPEM)
	if err != nil {
		return nil, err
	}

	if len(sshKnownHosts) > 0 {
		cfg.SSH.KnownHosts = sshKnownHosts
	}

	return &Backend{cfg: cfg, auth: auth}, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return TypeName
}

// Capabilities reports whole-snapshot export (ADR-0401).
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.SnapshotStore()
}

// Config exposes the resolved configuration (for connection tests).
func (b *Backend) Config() Config {
	return b.cfg
}

// Export writes payload at objectPath and pushes to the configured remote.
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	return Export(ctx, b.cfg, b.auth, payload, objectPath)
}
