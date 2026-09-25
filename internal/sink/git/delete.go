// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/platformrelay/kollect/internal/sink/objectstore"
)

// deleteCommitMessage overrides the export subject so a retraction commit is
// never mistaken for a snapshot update in git history (K-28).
const deleteCommitMessage = "chore({cluster}/{namespace}/{name}): remove inventory export"

// DeleteExport removes the inventory's exported files at the given candidate
// paths — each exact path plus its deterministic .part-NNNN-of-NNNN siblings —
// in a single commit and pushes. Missing candidates are not errors: cleanup is
// retried and must be idempotent.
func (b *Backend) DeleteExport(ctx context.Context, paths []string) ([]string, error) {
	commitCtx, ok := CommitContextFromContext(ctx)
	if !ok && len(paths) > 0 {
		commitCtx = CommitContextFromObjectPath(paths[0], b.cfg.Cluster)
	}

	return DeleteExportWithBranch(ctx, b.cfg, b.auth, paths, nil, commitCtx)
}

// DeleteExportWithBranch mirrors ExportFilesWithBranch's engine split for the
// deletion path (file:// or CLI engine -> git worktree + git add -A; otherwise
// go-git), reusing the same repo export lock so a delete can never race an
// in-flight export of the same branch.
func DeleteExportWithBranch(
	ctx context.Context,
	cfg Config,
	auth Auth,
	paths []string,
	branch *BranchSpec,
	commitCtx CommitContext,
) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	cfg = cfg.withDefaults()
	cfg.CommitMessage = deleteCommitMessage

	req, validated, err := validateDeletePaths(cfg, paths, branch)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()

	lockKey := cfg.Endpoint
	if lockKey == "" {
		lockKey = req.cloneURL
	}

	var deleted []string

	if isFileRemote(req.cloneURL) || cfg.Engine == GitEngineCLI {
		var deleteErr error
		if err := withRepoExportLock(lockKey, req.pushBranch, func() error {
			deleted, deleteErr = deleteViaCLI(ctx, cfg, auth, req, validated, commitCtx)

			return deleteErr
		}); err != nil {
			return nil, ClassifyExportError(err)
		}

		return deleted, ClassifyExportError(deleteErr)
	}

	var deleteErr error
	if err := withRepoExportLock(lockKey, req.pushBranch, func() error {
		deleted, deleteErr = deleteRemote(ctx, cfg, auth, req, validated, commitCtx)

		return deleteErr
	}); err != nil {
		return nil, ClassifyExportError(err)
	}

	return deleted, ClassifyExportError(deleteErr)
}

func validateDeletePaths(cfg Config, paths []string, branch *BranchSpec) (exportRequest, []string, error) {
	validated := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, objectPath := range paths {
		validatedPath, err := validateObjectPath(objectPath)
		if err != nil {
			return exportRequest{}, nil, fmt.Errorf("git cleanup: %w", err)
		}
		if validatedPath == "" {
			continue
		}
		if _, dup := seen[validatedPath]; dup {
			continue
		}
		seen[validatedPath] = struct{}{}
		validated = append(validated, validatedPath)
	}

	if len(validated) == 0 {
		return exportRequest{}, nil, nil
	}

	cloneURL, defaultBranch, err := parseRemote(cfg.Endpoint)
	if err != nil {
		return exportRequest{}, nil, err
	}

	if err = validateCloneURL(cloneURL); err != nil {
		return exportRequest{}, nil, fmt.Errorf("git cleanup: %w", err)
	}

	cloneBranch, pushBranch := resolveBranches(cfg.EffectiveBranch(defaultBranch), branch)

	if err = ValidateGitRef(cloneBranch); err != nil {
		return exportRequest{}, nil, fmt.Errorf("git cleanup: invalid clone branch: %w", err)
	}

	if err = ValidateGitRef(pushBranch); err != nil {
		return exportRequest{}, nil, fmt.Errorf("git cleanup: invalid push branch: %w", err)
	}

	return exportRequest{
		cloneURL:    cloneURL,
		cloneBranch: cloneBranch,
		pushBranch:  pushBranch,
		objectPath:  validated[0],
	}, validated, nil
}

func deleteViaCLI(
	ctx context.Context,
	cfg Config,
	auth Auth,
	req exportRequest,
	paths []string,
	commitCtx CommitContext,
) ([]string, error) {
	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	cli, err := newCLIEnv(cfg, auth, authType)
	if err != nil {
		return nil, err
	}
	defer cli.cleanup()
	if guardErr := cli.guardResolution(ctx, cfg.Endpoint); guardErr != nil {
		return nil, fmt.Errorf("git cleanup: %w", guardErr)
	}

	workdir, err := prepareMirrorWorkdir(ctx, cfg, auth, req.cloneURL, req.cloneBranch)
	if err != nil {
		return nil, err
	}
	if isFileRemote(req.cloneURL) {
		defer func() { _ = os.RemoveAll(workdir) }()
	}

	cloneURLForCLI := req.cloneURL
	if creds := auth.embedInURL(req.cloneURL); creds != "" && !cfg.ForceBasicAuth {
		cloneURLForCLI = creds
	}

	if err = prepareCLIWorkdir(ctx, workdir, cloneURLForCLI, req.cloneBranch, req.pushBranch, cfg, cli); err != nil {
		return nil, err
	}

	removed, err := removeDiskCandidates(workdir, paths)
	if err != nil {
		return nil, fmt.Errorf("git cleanup: %w", err)
	}

	if len(removed) > 0 {
		// Stage exactly the cleanup's own deletions (pathspec-scoped git add -A):
		// a shared warm mirror may hold unrelated dirt from a crashed export, and
		// it must not ride along under the deletion commit's subject.
		if err = gitAddPaths(ctx, workdir, removed, cli); err != nil {
			return nil, err
		}
	} else {
		// Nothing matched on disk. Sync only to deliver a deletion commit an
		// earlier crashed attempt left unpushed; for a branch that never held
		// work, syncCLIWorkdir would push a pointer-only feature branch that
		// downstream merge-request logic could mistake for a real deletion.
		nothing, probeErr := pushBranchWithoutWork(ctx, workdir, cli, req.cloneBranch, req.pushBranch)
		if probeErr != nil {
			return nil, fmt.Errorf("git cleanup: %w", probeErr)
		}
		if nothing {
			return nil, nil
		}
	}

	return removed, syncCLIWorkdir(ctx, workdir, req.cloneURL, req.pushBranch, cfg, commitCtx, cli)
}

// pushBranchWithoutWork reports a provably empty deletion for the CLI engine:
// a clean worktree whose HEAD equals the remote clone-branch tip while the push
// branch either does not exist remotely or already sits exactly at HEAD.
func pushBranchWithoutWork(
	ctx context.Context,
	workdir string,
	cli *cliEnv,
	cloneBranch, pushBranch string,
) (bool, error) {
	clean, err := gitStatusClean(ctx, workdir, cli)
	if err != nil {
		return false, err
	}
	if !clean {
		return false, nil
	}

	headOut, err := gitInWorkdir(ctx, workdir, cli, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git rev-parse HEAD: %s: %w", cli.redact(strings.TrimSpace(string(headOut))), err)
	}
	head := strings.TrimSpace(string(headOut))
	if head == "" {
		return false, nil
	}

	pushRef := "refs/heads/" + pushBranch
	pushOut, err := gitInWorkdir(ctx, workdir, cli, "ls-remote", "origin", pushRef).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git ls-remote origin %s: %s: %w", pushRef, cli.redact(strings.TrimSpace(string(pushOut))), err)
	}

	pushSHA := remoteSHAFromLsRemote(string(pushOut))
	if pushSHA != "" {
		return pushSHA == head, nil
	}

	cloneRef := "refs/heads/" + cloneBranch
	cloneOut, err := gitInWorkdir(ctx, workdir, cli, "ls-remote", "origin", cloneRef).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git ls-remote origin %s: %s: %w", cloneRef, cli.redact(strings.TrimSpace(string(cloneOut))), err)
	}

	return remoteSHAFromLsRemote(string(cloneOut)) == head, nil
}

// removeDiskCandidates deletes every worktree file matching the candidates'
// cleanup matchers (exact path + part siblings). Missing files are skipped.
func removeDiskCandidates(workdir string, paths []string) ([]string, error) {
	var removed []string

	for _, candidate := range paths {
		for _, m := range objectstore.CleanupMatchers(candidate) {
			listDir := m.ListDir()
			entries, err := os.ReadDir(filepath.Join(workdir, filepath.FromSlash(listDir)))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}

				return nil, err
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				full := objectstore.JoinDir(listDir, entry.Name())
				if !m.Matches(full) {
					continue
				}

				abs, relPath, pathErr := objectPathInWorkdir(workdir, full)
				if pathErr != nil {
					return nil, pathErr
				}

				if rmErr := os.Remove(abs); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
					return nil, rmErr
				}

				removed = append(removed, relPath)
			}
		}
	}

	return removed, nil
}

func deleteRemote(
	ctx context.Context,
	cfg Config,
	auth Auth,
	req exportRequest,
	paths []string,
	commitCtx CommitContext,
) ([]string, error) {
	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	sshCfg := cfg.effectiveSSHConfig()

	authMethod, guardedReq, err := guardedGoGitAuth(ctx, req, auth, authType, sshCfg, cfg.ForceBasicAuth)
	if err != nil {
		return nil, err
	}
	req = guardedReq

	workdir, err := prepareMirrorWorkdir(ctx, cfg, auth, req.cloneURL, req.cloneBranch)
	if err != nil {
		return nil, err
	}
	if isFileRemote(req.cloneURL) {
		defer func() { _ = os.RemoveAll(workdir) }()
	}

	repo, emptyRemote, err := openOrWarmMirror(ctx, workdir, req.cloneURL, req.cloneBranch, cfg.CloneDepth, authMethod, cfg)
	if err != nil {
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}

	if checkoutErr := checkoutMirrorBranch(wt, req.pushBranch); checkoutErr != nil {
		return nil, fmt.Errorf("checkout branch: %w", checkoutErr)
	}

	removed, err := removeWorktreeCandidates(wt, paths)
	if err != nil {
		return nil, err
	}

	if len(removed) == 0 {
		staged, dirty := stagedCleanupDeletions(wt)
		if !dirty {
			// Safe to report no-op only because openOrWarmMirror force-fetches
			// `+refs/heads/<branch>`: a crash between the deletion commit and its
			// push leaves the local ref behind on retry and the worktree dirty, so
			// a clean status here genuinely means the remote already lacks the files.
			return nil, nil
		}
		// A crashed earlier attempt staged its deletions in the persistent mirror
		// before committing: deliver them now.
		removed = staged
	}

	commitText := renderCommit(cfg, commitCtx)

	commit, err := wt.Commit(commitText.Full, &git.CommitOptions{
		Author: &object.Signature{
			Name:  cfg.Author.Name,
			Email: cfg.Author.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("git commit: %w", err)
	}

	if pushErr := pushCommitted(ctx, repo, cfg, authMethod, req.cloneURL, req.pushBranch, emptyRemote, commit, wt); pushErr != nil {
		return nil, pushErr
	}

	return removed, nil
}

// stagedCleanupDeletions lists staged deletions left by an interrupted cleanup
// (crash between wt.Remove and the commit) and whether the index holds any
// staged change at all. Untracked files are not staged changes: the cleanup
// must never commit foreign state out of the shared mirror.
func stagedCleanupDeletions(wt *git.Worktree) ([]string, bool) {
	status, err := wt.Status()
	if err != nil {
		return nil, false
	}

	var staged []string
	dirty := false

	for path, fs := range status {
		if fs.Staging == git.Untracked || fs.Staging == git.Unmodified {
			continue
		}
		dirty = true
		if fs.Staging == git.Deleted {
			staged = append(staged, path)
		}
	}

	sort.Strings(staged)

	return staged, dirty
}

func removeWorktreeCandidates(wt *git.Worktree, paths []string) ([]string, error) {
	var removed []string

	for _, candidate := range paths {
		for _, m := range objectstore.CleanupMatchers(candidate) {
			listDir := m.ListDir()
			entries, err := wt.Filesystem.ReadDir(listDir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}

				return nil, fmt.Errorf("cleanup read dir %q: %w", listDir, err)
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				full := objectstore.JoinDir(listDir, entry.Name())
				if !m.Matches(full) {
					continue
				}

				if rmErr := wt.Filesystem.Remove(full); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
					return nil, fmt.Errorf("cleanup remove %q: %w", full, rmErr)
				}

				if _, rmErr := wt.Remove(full); rmErr != nil {
					return nil, fmt.Errorf("git remove %q: %w", full, rmErr)
				}

				removed = append(removed, full)
			}
		}
	}

	return removed, nil
}
