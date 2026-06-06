// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"fmt"
	"path"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/git"
)

// Backend exports inventory payloads to a GitLab git remote.
type Backend struct {
	cfg  Config
	auth git.Auth
}

// NewBackend constructs a GitLab sink backend from spec, optional resolved CA PEM, and credentials.
func NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	caPEM []byte,
	auth git.Auth,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, caPEM)
	if err != nil {
		return nil, err
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

// Export writes payload at objectPath and pushes to the configured GitLab remote.
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	invNS, invName, err := inventoryFromObjectPath(objectPath)
	if err != nil {
		return err
	}

	var branchSpec *git.BranchSpec
	featureBranch := BranchNameForExport(b.cfg.MergeRequest.BranchPrefix, invNS, invName)
	if b.cfg.MergeRequest.Mode == MergeRequestModeBranchMR {
		branchSpec = &git.BranchSpec{
			PushBranch:  featureBranch,
			CloneBranch: b.cfg.MergeRequest.TargetBranch,
		}
	}

	commitCtx := git.CommitContextFromObjectPath(objectPath, "")
	if err := git.ExportWithBranch(
		ctx, b.cfg.GitConfig(), b.auth, payload, objectPath, branchSpec, commitCtx,
	); err != nil {
		return err
	}

	token := strings.TrimSpace(b.auth.Token)
	if token == "" {
		token = strings.TrimSpace(b.auth.Password)
	}

	return EnsureMergeRequest(ctx, b.cfg, b.cfg.MergeRequest, featureBranch, invNS, invName, token, strings.TrimSpace(b.auth.Username))
}

func inventoryFromObjectPath(objectPath string) (namespace, name string, err error) {
	clean := strings.Trim(path.Clean(objectPath), "/")
	parts := strings.Split(clean, "/")
	if len(parts) < 3 || parts[0] != "inventory" {
		return "", "", fmt.Errorf("gitlab export: unexpected object path %q", objectPath)
	}

	base := strings.TrimSuffix(parts[2], path.Ext(parts[2]))
	if base == "" {
		return "", "", fmt.Errorf("gitlab export: missing inventory name in %q", objectPath)
	}

	return parts[1], base, nil
}
