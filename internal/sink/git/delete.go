// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"

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

	var deleted []string

	if isFileRemote(req.cloneURL) || cfg.Engine == GitEngineCLI {
		var deleteErr error
		if err := withRepoExportLock(req.cloneURL, req.cloneBranch, func() error {
			deleted, deleteErr = deleteViaCLI(ctx, cfg, auth, req, validated, commitCtx)

			return deleteErr
		}); err != nil {
			return nil, ClassifyExportError(err)
		}

		return deleted, ClassifyExportError(deleteErr)
	}

	var deleteErr error
	if err := withRepoExportLock(req.cloneURL, req.cloneBranch, func() error {
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

	// Capture the local push-branch tip before prepareCLIWorkdir runs
	// `git checkout -B`, which resets a pre-existing branch to the mirror's
	// current HEAD (possibly another inventory's tip). Only a tip that
	// pre-existed is a stranded deletion commit this retry may deliver.
	pushBranchHash, pushBranchExisted, err := gitRefHash(ctx, workdir, req.pushBranch, cli)
	if err != nil {
		return nil, fmt.Errorf("git cleanup: %w", err)
	}

	cloneURLForCLI := req.cloneURL
	if creds := auth.embedInURL(req.cloneURL); creds != "" && !cfg.ForceBasicAuth {
		cloneURLForCLI = creds
	}

	if err = prepareCLIWorkdir(ctx, workdir, cloneURLForCLI, req.cloneBranch, req.pushBranch, cfg, cli); err != nil {
		return nil, err
	}

	// A shared warm mirror may hold dirt from a crashed operation: staged adds
	// or staged deletions in the index, modified tracked files, or untracked
	// leftover writes. The mirror is a cache, not user state — reset it to HEAD
	// (staged/unstaged tracked changes) and clean it (untracked files) before
	// touching it, so none of it can reach the deletion commit or wedge the
	// push with nothing-to-commit errors (K-28). A crash of THIS deletion
	// between staging and commit self-heals: the reset restores the files and
	// the retry re-deletes them; a deletion commit stranded at HEAD survives
	// the reset and is delivered below.
	if err = gitResetHard(ctx, workdir, cli); err != nil {
		return nil, err
	}
	if err = gitCleanFd(ctx, workdir, cli); err != nil {
		return nil, err
	}

	removed, err := removeDiskCandidates(workdir, paths)
	if err != nil {
		return nil, fmt.Errorf("git cleanup: %w", err)
	}

	if len(removed) > 0 {
		// Stage exactly the cleanup's own deletions (pathspec-scoped git add -A),
		// and commit only those paths (git commit -- <paths>): even if foreign
		// staged state survived the reset, it cannot ride along under the
		// deletion commit's subject.
		if err = gitAddPaths(ctx, workdir, removed, cli); err != nil {
			return nil, err
		}
	} else {
		if !pushBranchExisted {
			// No local tip for this push branch ever existed: there is no
			// stranded deletion commit to deliver, and the checkout above only
			// synthesized a pointer at another inventory's HEAD. Never push it.
			return nil, nil
		}

		headHash, headErr := gitHeadHash(ctx, workdir, cli)
		if headErr != nil {
			return nil, fmt.Errorf("git cleanup: %w", headErr)
		}
		if headHash != pushBranchHash {
			// `git checkout -B` reset the pre-existing branch to an unrelated
			// HEAD; the stranded tip is gone, so never push that HEAD.
			return nil, nil
		}

		nothing, remotePushSHA, probeErr := pushBranchWithoutWork(ctx, workdir, cli, req.cloneBranch, req.pushBranch)
		if probeErr != nil {
			return nil, fmt.Errorf("git cleanup: %w", probeErr)
		}
		if nothing {
			return nil, nil
		}

		// The stranded tip is only deliverable as a fast-forward: a remote push
		// branch that has diverged from HEAD must not be overwritten.
		if remotePushSHA != "" {
			fastForward, ffErr := gitIsAncestorOfHead(ctx, workdir, remotePushSHA, cli)
			if ffErr != nil {
				return nil, fmt.Errorf("git cleanup: %w", ffErr)
			}
			if !fastForward {
				return nil, nil
			}
		}
	}

	return removed, syncCLIWorkdirScoped(ctx, workdir, req.cloneURL, req.pushBranch, cfg, commitCtx, cli, removed)
}

// pushBranchWithoutWork reports a provably empty deletion for the CLI engine:
// a clean worktree whose HEAD equals the remote clone-branch tip while the push
// branch either does not exist remotely or already sits exactly at HEAD. It
// also returns the remote push-branch tip ("" when the branch is absent) so the
// caller can gate a stranded-tip delivery on fast-forwardability.
func pushBranchWithoutWork(
	ctx context.Context,
	workdir string,
	cli *cliEnv,
	cloneBranch, pushBranch string,
) (bool, string, error) {
	clean, err := gitStatusClean(ctx, workdir, cli)
	if err != nil {
		return false, "", err
	}
	if !clean {
		return false, "", nil
	}

	headOut, err := gitInWorkdir(ctx, workdir, cli, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("git rev-parse HEAD: %s: %w", cli.redact(strings.TrimSpace(string(headOut))), err)
	}
	head := strings.TrimSpace(string(headOut))
	if head == "" {
		return false, "", nil
	}

	pushRef := "refs/heads/" + pushBranch
	pushOut, err := gitInWorkdir(ctx, workdir, cli, "ls-remote", "origin", pushRef).CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("git ls-remote origin %s: %s: %w", pushRef, cli.redact(strings.TrimSpace(string(pushOut))), err)
	}

	pushSHA := remoteSHAFromLsRemote(string(pushOut))
	if pushSHA != "" {
		return pushSHA == head, pushSHA, nil
	}

	cloneRef := "refs/heads/" + cloneBranch
	cloneOut, err := gitInWorkdir(ctx, workdir, cli, "ls-remote", "origin", cloneRef).CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("git ls-remote origin %s: %s: %w", cloneRef, cli.redact(strings.TrimSpace(string(cloneOut))), err)
	}

	return remoteSHAFromLsRemote(string(cloneOut)) == head, "", nil
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

	// Mirror dirt from a crashed operation (staged adds or deletions, modified
	// tracked files, untracked leftover writes) must never reach the deletion
	// commit or block the checkout: hard-reset the mirror to its HEAD first.
	// A crash of THIS deletion between wt.Remove and the commit self-heals —
	// the reset restores the files and the retry re-deletes them; a deletion
	// commit stranded at HEAD survives the reset (it never moves HEAD) and is
	// delivered by the stranded-tip check below.
	if head, headErr := repo.Head(); headErr == nil {
		if resetErr := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: head.Hash()}); resetErr != nil {
			return nil, fmt.Errorf("mirror reset: %w", resetErr)
		}
	}

	// Capture the local push-branch tip before checkout: a branch that did not
	// pre-exist only gets synthesized at the mirror's current HEAD (possibly
	// another inventory's tip), which must never be delivered as this branch's
	// deletion.
	pushBranchExisted := false
	if _, refErr := repo.Reference(plumbing.NewBranchReferenceName(req.pushBranch), true); refErr == nil {
		pushBranchExisted = true
	} else if !errors.Is(refErr, plumbing.ErrReferenceNotFound) {
		return nil, fmt.Errorf("resolve push branch: %w", refErr)
	}

	if checkoutErr := checkoutMirrorBranch(wt, req.pushBranch); checkoutErr != nil {
		return nil, fmt.Errorf("checkout branch: %w", checkoutErr)
	}

	removed, err := removeWorktreeCandidates(wt, paths)
	if err != nil {
		return nil, err
	}

	if len(removed) == 0 {
		if !pushBranchExisted {
			// No local tip for this push branch ever existed: there is no
			// stranded deletion commit to deliver, and the checkout above only
			// synthesized a pointer at another inventory's HEAD. Never push it.
			return nil, nil
		}

		synced, probeErr := pushBranchSynced(ctx, repo, cfg, authMethod, req.cloneBranch, req.pushBranch)
		if probeErr != nil {
			return nil, fmt.Errorf("git cleanup: %w", probeErr)
		}
		if synced {
			// The remote push branch already holds our HEAD: nothing was ever
			// exported (or a prior attempt fully delivered). Not a bare no-op
			// claim: the worktree matches HEAD, and HEAD equals the remote
			// push tip, so the remote genuinely lacks the candidate paths.
			return nil, nil
		}

		// A deletion commit stranded by a crash between commit and push: the
		// local push-branch tip already carries the deletion, so deliver that
		// tip without authoring a new commit. openOrWarmMirror force-fetches
		// the CLONE branch only — in branchMR mode the push branch ref is
		// never reconciled, which is exactly why this state survives a crash
		// and must be handled here (K-28). A push branch absent remotely that
		// holds no work (HEAD equals the remote clone tip) was already
		// excluded by the synced probe, so pushing here never creates a
		// pointer-only feature branch. Same non-fast-forward recovery as a
		// fresh deletion commit.
		head, headErr := repo.Head()
		if headErr != nil {
			return nil, fmt.Errorf("head: %w", headErr)
		}

		if pushErr := pushCommitted(ctx, repo, cfg, authMethod, req.cloneURL, req.pushBranch, emptyRemote, head.Hash(), wt); pushErr != nil {
			return nil, pushErr
		}

		return nil, nil
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

// pushBranchSynced reports whether the remote already holds the local
// push-branch tip, mirroring the CLI engine's pushBranchWithoutWork probe for
// the go-git engine: a push branch absent remotely counts as synced only when
// HEAD equals the remote clone tip, so a pointer-only branch that never held
// work is never pushed. The remote list runs once and covers both branches.
func pushBranchSynced(
	ctx context.Context,
	repo *git.Repository,
	cfg Config,
	authMethod transport.AuthMethod,
	cloneBranch, pushBranch string,
) (bool, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("remote origin: %w", err)
	}

	refs, listErr := remote.ListContext(ctx, &git.ListOptions{
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	})
	if listErr != nil {
		return false, fmt.Errorf("git remote list: %w", listErr)
	}

	tips := make(map[string]plumbing.Hash, len(refs))
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			tips[ref.Name().Short()] = ref.Hash()
		}
	}

	head, headErr := repo.Head()
	if headErr != nil {
		return false, fmt.Errorf("head: %w", headErr)
	}

	if pushTip, exists := tips[pushBranch]; exists {
		return pushTip == head.Hash(), nil
	}

	cloneTip, exists := tips[cloneBranch]

	return exists && cloneTip == head.Hash(), nil
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
