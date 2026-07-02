// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package local

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
)

// Backend writes export payloads to a local output directory.
type Backend struct {
	cfg Config
}

// NewBackend constructs a local filesystem sink backend from spec.
// The second parameter (resolved secret data) is accepted for Factory-signature
// compatibility but unused: the local sink requires no credentials.
func NewBackend(spec kollectdevv1alpha1.KollectSinkSpec, _ map[string][]byte) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec)
	if err != nil {
		return nil, err
	}

	return &Backend{cfg: cfg}, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return TypeName
}

// Capabilities reports whole-snapshot export (ADR-0401): each export overwrites
// the file at path in full, matching Git and other snapshot-store backends.
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.SnapshotStore()
}

// Export writes payload to outputDir/path, skipping the write if the existing
// file already has identical content (avoids spurious CI git diffs).
func (b *Backend) Export(_ context.Context, payload []byte, path string) error {
	full, err := b.safeJoin(path)
	if err != nil {
		return err
	}

	if identical, err := fileIdentical(full, payload); err == nil && identical {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { //nolint:gosec // dirs must be traversable, no secrets stored here
		return fmt.Errorf("local sink: create parent dirs for %q: %w", full, err)
	}

	if err := os.WriteFile(full, payload, 0o644); err != nil { //nolint:gosec // exported inventory is not sensitive; matches other snapshot sinks
		return fmt.Errorf("local sink: write %q: %w", full, err)
	}

	return nil
}

// safeJoin resolves path under b.cfg.OutputDir and rejects any result that
// escapes the output directory (path traversal via "../" or an absolute path).
func (b *Backend) safeJoin(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("local sink: path %q must be relative, got an absolute path", path)
	}

	base := filepath.Clean(b.cfg.OutputDir)
	full := filepath.Clean(filepath.Join(base, path))

	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("local sink: path %q escapes output directory", path)
	}

	return full, nil
}

func fileIdentical(path string, payload []byte) (bool, error) {
	existing, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from the sink's own configured output dir + sanitized template
	if err != nil {
		return false, err
	}

	return bytes.Equal(existing, payload), nil
}
