// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/pathvalidate"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
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
	invNS, invName := pathvalidate.InventoryFromObjectPath(objectPath)

	var branchSpec *git.BranchSpec
	featureBranch := BranchNameForExport(b.cfg.MergeRequest.BranchPrefix, invNS, invName)
	if b.cfg.MergeRequest.Mode == MergeRequestModeBranchMR {
		branchSpec = &git.BranchSpec{
			PushBranch:  featureBranch,
			CloneBranch: b.cfg.MergeRequest.TargetBranch,
		}
	}

	commitCtx, ok := git.CommitContextFromContext(ctx)
	if !ok {
		commitCtx = git.CommitContextFromObjectPath(objectPath, b.cfg.GitConfig().Cluster)
	}

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

// ExportFiles writes a projected layout tree in a single commit and pushes to GitLab (ADR-0419).
// opts carries prune intent: PruneKeepPaths overrides the keep-set (multipart union) and
// SuppressPrune forces prune off (non-final multipart part) so prune runs exactly once.
func (b *Backend) ExportFiles(ctx context.Context, files []git.FileEntry, opts git.ExportFilesOptions) error {
	if len(files) == 0 {
		return fmt.Errorf("gitlab export: no files to write")
	}

	commitCtx, ok := git.CommitContextFromContext(ctx)
	if !ok {
		commitCtx = git.CommitContextFromObjectPath(files[0].Path, b.cfg.GitConfig().Cluster)
	}

	invNS, invName := commitCtx.Namespace, commitCtx.Name

	var branchSpec *git.BranchSpec
	featureBranch := BranchNameForExport(b.cfg.MergeRequest.BranchPrefix, invNS, invName)
	if b.cfg.MergeRequest.Mode == MergeRequestModeBranchMR {
		branchSpec = &git.BranchSpec{
			PushBranch:  featureBranch,
			CloneBranch: b.cfg.MergeRequest.TargetBranch,
		}
	}

	cfg := b.cfg.GitConfig()
	cfg.Prune = (cfg.Prune || opts.Prune) && !opts.SuppressPrune
	cfg.PruneKeepPaths = opts.PruneKeepPaths

	if err := git.ExportFilesWithBranch(ctx, cfg, b.auth, files, branchSpec, commitCtx); err != nil {
		return err
	}

	token := strings.TrimSpace(b.auth.Token)
	if token == "" {
		token = strings.TrimSpace(b.auth.Password)
	}

	return EnsureMergeRequest(ctx, b.cfg, b.cfg.MergeRequest, featureBranch, invNS, invName, token, strings.TrimSpace(b.auth.Username))
}

// DeleteExport removes the inventory's exported files in a deletion commit on the
// configured branch (feature branch + MR in branchMR mode) (K-28, C-2a). With
// branchMR it opens the merge request only when the deletion actually changed the
// branch: opening an MR whose source branch never received a commit can never
// succeed (GitLab rejects a missing source branch) and would wedge the cleanup
// retry loop while the inventory is Terminating.
func (b *Backend) DeleteExport(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	commitCtx, ok := git.CommitContextFromContext(ctx)
	if !ok {
		commitCtx = git.CommitContextFromObjectPath(paths[0], b.cfg.GitConfig().Cluster)
	}

	invNS, invName := commitCtx.Namespace, commitCtx.Name

	var branchSpec *git.BranchSpec
	featureBranch := BranchNameForExport(b.cfg.MergeRequest.BranchPrefix, invNS, invName)
	if b.cfg.MergeRequest.Mode == MergeRequestModeBranchMR {
		branchSpec = &git.BranchSpec{
			PushBranch:  featureBranch,
			CloneBranch: b.cfg.MergeRequest.TargetBranch,
		}
	}

	deleted, delErr := git.DeleteExportWithBranch(ctx, b.cfg.GitConfig(), b.auth, paths, branchSpec, commitCtx)
	if delErr != nil {
		return deleted, delErr
	}

	if b.cfg.MergeRequest.Mode != MergeRequestModeBranchMR {
		return deleted, nil
	}

	branchHasWork := len(deleted) > 0
	if !branchHasWork {
		// Either nothing was ever exported (no branch: nothing to merge) or a
		// branch is present that may hold a deletion commit an earlier attempt
		// pushed but died before opening the MR for. The probe cannot tell a
		// stranded deletion commit from a stale unmerged export commit — reopening
		// is safe only because branchMR cleanup always announces retention
		// (sink.RunCleanupExport mrMediated), so the operator, not the controller,
		// decides what the opened MR finally merges. A failed lookup is an
		// unknown, not an absence: return it so cleanup retries.
		exists, probeErr := git.RemoteBranchExists(ctx, b.cfg.GitConfig(), b.auth, featureBranch)
		if probeErr != nil {
			return nil, probeErr
		}
		branchHasWork = exists
	}
	if !branchHasWork {
		return nil, nil
	}

	token := strings.TrimSpace(b.auth.Token)
	if token == "" {
		token = strings.TrimSpace(b.auth.Password)
	}

	return deleted, EnsureMergeRequest(ctx, b.cfg, b.cfg.MergeRequest, featureBranch, invNS, invName, token, strings.TrimSpace(b.auth.Username))
}
