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

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
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
// retried and must be idempotent. Without an owner it never removes a path an
// ownership record lists (ADR-0422); ReleaseExport is the owner-aware form.
func (b *Backend) DeleteExport(ctx context.Context, paths []string) ([]string, error) {
	commitCtx, ok := CommitContextFromContext(ctx)
	if !ok && len(paths) > 0 {
		commitCtx = CommitContextFromObjectPath(paths[0], b.cfg.Cluster)
	}

	return DeleteExportWithBranch(ctx, b.cfg, b.auth, paths, nil, commitCtx)
}

// ReleaseOptions names the deleting inventory for an ownership-aware deletion (ADR-0422).
//
// Owner is the deleting inventory's prune owner (InventoryPruneOwner); its ownership record is
// removed. KeepFiles (deletionPolicy Retain, or a retraction skipped for a shared export identity)
// removes only that record; otherwise the candidate paths and every path the owner's record lists
// are removed too. Paths another owner's record lists are never removed.
type ReleaseOptions struct {
	Owner     string
	KeepFiles bool
}

// ReleaseExport is DeleteExport for a known owner: it releases the owner's ownership record and,
// unless opts.KeepFiles, retracts the candidate paths and the owner's recorded files, in one commit.
// paths also name the inventory for the commit subject when no commit context is attached.
func (b *Backend) ReleaseExport(ctx context.Context, paths []string, opts ReleaseOptions) ([]string, error) {
	commitCtx, ok := CommitContextFromContext(ctx)
	if !ok && len(paths) > 0 {
		commitCtx = CommitContextFromObjectPath(paths[0], b.cfg.Cluster)
	}

	return DeleteExportWithBranch(ctx, ReleaseConfig(b.cfg, opts), b.auth, paths, nil, commitCtx)
}

// ReleaseConfig applies opts to a deletion's config.
func ReleaseConfig(cfg Config, opts ReleaseOptions) Config {
	cfg.PruneOwner = opts.Owner
	cfg.ReleaseOnly = opts.KeepFiles

	return cfg
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
	if len(paths) == 0 && cfg.PruneOwner == "" {
		return nil, nil
	}

	cfg = deletionConfig(cfg)

	req, validated, err := validateDeletePaths(cfg, paths, branch)
	if err != nil {
		return nil, err
	}
	if len(validated) == 0 && cfg.PruneOwner == "" {
		return nil, nil
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

// deletionConfig defaults cfg for a deletion and selects its commit subject: a release that keeps
// the files is not a retraction.
func deletionConfig(cfg Config) Config {
	cfg = cfg.withDefaults()
	cfg.CommitMessage = deleteCommitMessage
	if cfg.ReleaseOnly {
		cfg.CommitMessage = releaseCommitMessage
	}

	return cfg
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

	if len(validated) == 0 && cfg.PruneOwner == "" {
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

	objectPath := pruneRecordPath(cfg.PruneOwner)
	if len(validated) > 0 {
		objectPath = validated[0]
	}

	return exportRequest{
		cloneURL:    cloneURL,
		cloneBranch: cloneBranch,
		pushBranch:  pushBranch,
		objectPath:  objectPath,
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

	// Capture the local push-branch tip before prepareCLIWorkdir checks the
	// branch out. A pre-existing branch is checked out without resetting, so
	// this tip survives; only a tip that pre-existed is a stranded deletion
	// commit this retry may deliver.
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

	if err = realignDivergedDirectBranch(ctx, workdir, req, cli); err != nil {
		return nil, fmt.Errorf("git cleanup: %w", err)
	}

	if !pushBranchExisted {
		if err = baseOnRemotePushBranch(ctx, workdir, req, cfg.CloneDepth, cli); err != nil {
			return nil, fmt.Errorf("git cleanup: %w", err)
		}
	}

	removed, err := releaseOnDisk(workdir, cfg, paths)
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
		deliver, deliverErr := cliStrandedDeliveryDue(ctx, workdir, req, cli, pushBranchHash, pushBranchExisted)
		if deliverErr != nil {
			return nil, deliverErr
		}
		if !deliver {
			return nil, nil
		}
	}

	return removed, syncCLIWorkdirScoped(ctx, workdir, req.cloneURL, req.pushBranch, cfg, commitCtx, cli, removed)
}

// baseOnRemotePushBranch handles a merge-request feature branch the mirror does
// not hold locally (a cold mirror: a restarted pod, or a file:// remote, which
// gets a fresh workdir per operation). prepareCLIWorkdir synthesized it from the
// target branch, where an unmerged export is absent, so the deletion would find
// nothing while the remote feature branch — and its merge request — still adds
// the inventory's snapshot. When the remote holds the feature branch, base the
// deletion on its tip instead, so the retraction lands on that branch as a
// fast-forward. Direct mode (push == clone) is untouched.
func baseOnRemotePushBranch(ctx context.Context, workdir string, req exportRequest, depth int, cli *cliEnv) error {
	if req.pushBranch == req.cloneBranch {
		return nil
	}

	pushRef := "refs/heads/" + req.pushBranch
	out, err := gitInWorkdir(ctx, workdir, cli, "ls-remote", "origin", pushRef).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git ls-remote origin %s: %s: %w", pushRef, cli.redact(strings.TrimSpace(string(out))), err)
	}

	remoteTip := remoteSHAFromLsRemote(string(out))
	if remoteTip == "" {
		return nil
	}

	if err = gitFetchShallow(ctx, workdir, req.pushBranch, depth, cli); err != nil {
		return err
	}

	return gitResetHardTo(ctx, workdir, remoteTip, cli)
}

// realignDivergedDirectBranch handles direct mode (push branch == clone branch)
// for the CLI engine. `git fetch` only moves origin/<branch>; the mirror's local
// branch keeps whatever an earlier crashed attempt committed. When the remote
// advanced past such a stranded commit, it can no longer be delivered as a
// fast-forward, and the deletion would see its own files already gone locally
// and report a no-op over a remote that still holds them. Reset the local
// branch to the fetched remote tip instead, so the candidates are removed again
// on top of the remote's history (the go-git engine gets the same effect from
// its forced fetch refspec). A local branch the remote tip is an ancestor of —
// a stranded commit that still fast-forwards — is kept and delivered.
func realignDivergedDirectBranch(ctx context.Context, workdir string, req exportRequest, cli *cliEnv) error {
	if req.pushBranch != req.cloneBranch {
		return nil
	}

	remoteRef := "refs/remotes/origin/" + req.cloneBranch

	remoteTip, exists, err := gitRevParse(ctx, workdir, remoteRef, cli)
	if err != nil || !exists {
		return err
	}

	fastForward, err := gitIsAncestorOfHead(ctx, workdir, remoteTip, cli)
	if err != nil || fastForward {
		return err
	}

	return gitResetHardTo(ctx, workdir, remoteTip, cli)
}

// cliStrandedDeliveryDue decides whether the CLI engine may deliver a deletion
// commit stranded on a pre-existing push branch. It returns true only for a
// genuine fast-forward delivery: the push-branch ref pre-existed the operation's
// checkout, HEAD still equals that captured tip, and the remote push tip exists
// and is an ancestor of HEAD. Every other state — no local tip, a diverged or
// absent remote, or a branch poisoned by another inventory's crashed export — is
// a no-op that must never push.
func cliStrandedDeliveryDue(
	ctx context.Context,
	workdir string,
	req exportRequest,
	cli *cliEnv,
	pushBranchHash string,
	pushBranchExisted bool,
) (bool, error) {
	if !pushBranchExisted {
		// No local tip for this push branch ever existed: there is no
		// stranded deletion commit to deliver, and the checkout above only
		// synthesized a pointer at the clone tip. Never push it.
		return false, nil
	}

	headHash, headErr := gitHeadHash(ctx, workdir, cli)
	if headErr != nil {
		return false, fmt.Errorf("git cleanup: %w", headErr)
	}
	if headHash != pushBranchHash {
		// Defensive: the pre-existing tip no longer matches HEAD, so there
		// is no stranded deletion commit to deliver. Never push.
		return false, nil
	}

	nothing, remotePushSHA, probeErr := pushBranchWithoutWork(ctx, workdir, cli, req.cloneBranch, req.pushBranch)
	if probeErr != nil {
		return false, fmt.Errorf("git cleanup: %w", probeErr)
	}
	if nothing {
		return false, nil
	}

	// Deliver only when the remote push branch exists and its tip is an
	// ancestor of HEAD: a pure fast-forward. A remote tip that is absent,
	// diverged, or unrelated to this inventory's work is a no-op.
	if remotePushSHA == "" {
		return false, nil
	}
	fastForward, ffErr := gitIsAncestorOfHead(ctx, workdir, remotePushSHA, cli)
	if ffErr != nil {
		return false, fmt.Errorf("git cleanup: %w", ffErr)
	}
	if !fastForward {
		return false, nil
	}

	return true, nil
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

// releaseOnDisk removes what planRelease decides from the CLI engine's worktree:
// the matched candidates no other owner records, then the owner's recorded
// files and its record (ADR-0422).
func releaseOnDisk(workdir string, cfg Config, paths []string) ([]string, error) {
	fs := osfs.New(workdir)
	plan, err := planRelease(fs, cfg, paths)
	if err != nil {
		return nil, err
	}

	removed, err := removeDiskCandidates(workdir, plan.candidates, plan.protected)
	if err != nil {
		return nil, err
	}

	exact, err := removeExact(fs, plan.exact, nil)
	if err != nil {
		return nil, err
	}

	return appendNew(removed, exact), nil
}

// removeDiskCandidates deletes every worktree file matching the candidates'
// cleanup matchers (exact path + part siblings), except a path protected lists
// (another inventory's recorded file). Missing files are skipped.
func removeDiskCandidates(workdir string, paths []string, protected map[string]string) ([]string, error) {
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
				if _, foreign := protected[full]; foreign {
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
	// pre-exist is synthesized at the just-fetched clone tip, so it is never
	// delivered as this branch's deletion.
	pushBranchExisted := false
	if _, refErr := repo.Reference(plumbing.NewBranchReferenceName(req.pushBranch), true); refErr == nil {
		pushBranchExisted = true
	} else if !errors.Is(refErr, plumbing.ErrReferenceNotFound) {
		return nil, fmt.Errorf("resolve push branch: %w", refErr)
	}

	if !pushBranchExisted {
		if fetchErr := fetchRemotePushBranch(ctx, repo, cfg, authMethod, req); fetchErr != nil {
			return nil, fmt.Errorf("git cleanup: %w", fetchErr)
		}
	}

	if checkoutErr := checkoutMirrorBranch(repo, wt, req.cloneBranch, req.pushBranch); checkoutErr != nil {
		return nil, fmt.Errorf("checkout branch: %w", checkoutErr)
	}

	removed, err := releaseInWorktree(wt, cfg, paths)
	if err != nil {
		return nil, err
	}

	if len(removed) == 0 {
		return nil, deliverRemoteStrandedDeletion(ctx, repo, cfg, authMethod, req, wt, emptyRemote, pushBranchExisted)
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

// fetchRemotePushBranch is the go-git counterpart of baseOnRemotePushBranch: a
// merge-request feature branch missing from the mirror but present on the
// remote is fetched into the local branch ref, so checkoutMirrorBranch checks
// it out as-is and the deletion lands on it instead of on a branch synthesized
// from the target branch (where an unmerged export is absent).
func fetchRemotePushBranch(
	ctx context.Context,
	repo *git.Repository,
	cfg Config,
	authMethod transport.AuthMethod,
	req exportRequest,
) error {
	if req.pushBranch == req.cloneBranch {
		return nil
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin: %w", err)
	}

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	})
	if err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}

		return fmt.Errorf("git remote list: %w", err)
	}

	pushRef := plumbing.NewBranchReferenceName(req.pushBranch)
	found := false
	for _, ref := range refs {
		if ref.Name() == pushRef {
			found = true

			break
		}
	}
	if !found {
		return nil
	}

	fetchErr := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName:      "origin",
		RefSpecs:        []config.RefSpec{config.RefSpec(fmt.Sprintf("+%s:%s", pushRef, pushRef))},
		Depth:           cfg.CloneDepth,
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch push branch: %w", fetchErr)
	}

	return nil
}

// deliverRemoteStrandedDeletion handles the nothing-matched case for the go-git
// engine: it either reports a no-op or pushes a deletion commit stranded on a
// pre-existing push branch. It pushes only for a genuine fast-forward delivery;
// every other state is a no-op that must never push.
func deliverRemoteStrandedDeletion(
	ctx context.Context,
	repo *git.Repository,
	cfg Config,
	authMethod transport.AuthMethod,
	req exportRequest,
	wt *git.Worktree,
	emptyRemote, pushBranchExisted bool,
) error {
	if !pushBranchExisted {
		// No local tip for this push branch ever existed: there is no
		// stranded deletion commit to deliver, and the checkout above only
		// synthesized a pointer at the clone tip. Never push it.
		return nil
	}

	synced, pushTip, pushExists, probeErr := pushBranchSynced(ctx, repo, cfg, authMethod, req.cloneBranch, req.pushBranch)
	if probeErr != nil {
		return fmt.Errorf("git cleanup: %w", probeErr)
	}
	if synced {
		// The remote push branch already holds our HEAD: nothing was ever
		// exported (or a prior attempt fully delivered). Not a bare no-op
		// claim: the worktree matches HEAD, and HEAD equals the remote
		// push tip, so the remote genuinely lacks the candidate paths.
		return nil
	}

	// Deliver a stranded deletion commit only when the remote push branch
	// exists and its tip is an ancestor of HEAD: a pure fast-forward. A
	// pre-existing local branch pointing at another inventory's tip (a
	// crashed foreign export or a prior no-op delete that persisted the
	// synthesized ref) is not this inventory's work and must never be
	// pushed. The remote list runs once and yields the push tip.
	if !pushExists {
		return nil
	}

	head, headErr := repo.Head()
	if headErr != nil {
		return fmt.Errorf("head: %w", headErr)
	}
	if !remoteTipFastForwardable(repo, pushTip, head.Hash()) {
		return nil
	}

	return pushCommitted(ctx, repo, cfg, authMethod, req.cloneURL, req.pushBranch, emptyRemote, head.Hash(), wt)
}

// pushBranchSynced reports whether the remote already holds the local
// push-branch tip, mirroring the CLI engine's pushBranchWithoutWork probe for
// the go-git engine: a push branch absent remotely counts as synced only when
// HEAD equals the remote clone tip, so a pointer-only branch that never held
// work is never pushed. The remote list runs once and covers both branches; it
// also yields the remote push tip (when present) so the caller can require a
// pure fast-forward before delivering a stranded tip.
func pushBranchSynced(
	ctx context.Context,
	repo *git.Repository,
	cfg Config,
	authMethod transport.AuthMethod,
	cloneBranch, pushBranch string,
) (bool, plumbing.Hash, bool, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return false, plumbing.ZeroHash, false, fmt.Errorf("remote origin: %w", err)
	}

	refs, listErr := remote.ListContext(ctx, &git.ListOptions{
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	})
	if listErr != nil {
		return false, plumbing.ZeroHash, false, fmt.Errorf("git remote list: %w", listErr)
	}

	tips := make(map[string]plumbing.Hash, len(refs))
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			tips[ref.Name().Short()] = ref.Hash()
		}
	}

	head, headErr := repo.Head()
	if headErr != nil {
		return false, plumbing.ZeroHash, false, fmt.Errorf("head: %w", headErr)
	}

	if pushTip, exists := tips[pushBranch]; exists {
		return pushTip == head.Hash(), pushTip, true, nil
	}

	cloneTip, exists := tips[cloneBranch]

	return exists && cloneTip == head.Hash(), plumbing.ZeroHash, false, nil
}

// remoteTipFastForwardable reports whether remoteTip is an ancestor of head,
// i.e. head can be pushed to that branch as a fast-forward. A remote tip whose
// commit is not present locally, or whose ancestry cannot be established, is
// reported as false so the delivery is a no-op instead of forcing unrelated
// remote state.
func remoteTipFastForwardable(repo *git.Repository, remoteTip, head plumbing.Hash) bool {
	if remoteTip == head {
		return true
	}

	remoteCommit, err := repo.CommitObject(remoteTip)
	if err != nil {
		return false
	}

	headCommit, err := repo.CommitObject(head)
	if err != nil {
		return false
	}

	ancestor, err := remoteCommit.IsAncestor(headCommit)
	if err != nil {
		return false
	}

	return ancestor
}

// releaseInWorktree is releaseOnDisk for the go-git engine: removals are staged
// in the index as they happen.
func releaseInWorktree(wt *git.Worktree, cfg Config, paths []string) ([]string, error) {
	return releaseFS(wt.Filesystem, cfg, paths, func(p string) (bool, error) {
		if _, rmErr := wt.Remove(p); rmErr != nil {
			if errors.Is(rmErr, index.ErrEntryNotFound) {
				// Untracked leftover dirt: the disk removal is the whole
				// effect, so it stages no index change and is not reported as
				// a removal the caller may commit.
				return false, nil
			}

			return false, fmt.Errorf("git remove %q: %w", p, rmErr)
		}

		return true, nil
	})
}

// releaseFS plans and applies a release on a billy worktree; stage records each
// removal in the index (nil: no index, as for the in-memory test twin).
func releaseFS(fs billy.Filesystem, cfg Config, paths []string, stage func(string) (bool, error)) ([]string, error) {
	plan, err := planRelease(fs, cfg, paths)
	if err != nil {
		return nil, err
	}

	removed, err := removeFSCandidates(fs, plan.candidates, plan.protected, stage)
	if err != nil {
		return nil, err
	}

	exact, err := removeExact(fs, plan.exact, stage)
	if err != nil {
		return nil, err
	}

	return appendNew(removed, exact), nil
}

// removeWorktreeCandidates deletes and unstages every worktree file matching
// the candidates' cleanup matchers, except a path protected lists.
func removeWorktreeCandidates(wt *git.Worktree, paths []string, protected map[string]string) ([]string, error) {
	return removeFSCandidates(wt.Filesystem, paths, protected, func(p string) (bool, error) {
		if _, rmErr := wt.Remove(p); rmErr != nil {
			if errors.Is(rmErr, index.ErrEntryNotFound) {
				return false, nil
			}

			return false, fmt.Errorf("git remove %q: %w", p, rmErr)
		}

		return true, nil
	})
}

// removeFSCandidates deletes every file matching the candidates' cleanup
// matchers (exact path + part siblings), except a path protected lists
// (another inventory's recorded file). stage, when set, records the removal and
// reports false for a path the index never tracked; such a path is not
// reported as removed.
func removeFSCandidates(
	fs billy.Filesystem,
	paths []string,
	protected map[string]string,
	stage func(string) (bool, error),
) ([]string, error) {
	var removed []string

	for _, candidate := range paths {
		for _, m := range objectstore.CleanupMatchers(candidate) {
			listDir := m.ListDir()
			entries, err := fs.ReadDir(listDir)
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
				if _, foreign := protected[full]; foreign {
					continue
				}

				if rmErr := fs.Remove(full); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
					return nil, fmt.Errorf("cleanup remove %q: %w", full, rmErr)
				}

				if stage != nil {
					tracked, stageErr := stage(full)
					if stageErr != nil {
						return nil, stageErr
					}
					if !tracked {
						continue
					}
				}

				removed = append(removed, full)
			}
		}
	}

	return removed, nil
}
