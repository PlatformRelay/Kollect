// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	defaultBranch    = "main"
	defaultObjectKey = "inventory/latest.json"
	exportTimeout    = 2 * time.Minute
)

type BranchSpec struct {
	PushBranch  string
	CloneBranch string
}

func Export(ctx context.Context, cfg Config, auth Auth, payload []byte, objectPath string) error {
	commitCtx, ok := CommitContextFromContext(ctx)
	if !ok {
		commitCtx = CommitContextFromObjectPath(objectPath, cfg.Cluster)
	}

	return ExportWithBranch(ctx, cfg, auth, payload, objectPath, nil, commitCtx)
}

func ExportWithBranch(
	ctx context.Context,
	cfg Config,
	auth Auth,
	payload []byte,
	objectPath string,
	branch *BranchSpec,
	commitCtx CommitContext,
) error {
	if len(payload) == 0 {
		return fmt.Errorf("git export: empty payload")
	}

	return ExportFilesWithBranch(ctx, cfg, auth, []FileEntry{{Path: objectPath, Data: payload}}, branch, commitCtx)
}

// FileEntry is one repo file written in a multi-file git export (ADR-0419 layout tree).
type FileEntry struct {
	// Path is the repo-relative slash path.
	Path string
	// Data is the file content.
	Data []byte
}

// ExportFilesWithBranch writes a set of files in a single commit and pushes to the remote (ADR-0419).
// Document-mode exports pass one file; perResource/split layouts pass the projected tree. Prune
// (cfg.Prune) stages deletions of stale files so removed resources drop out of the repo.
func ExportFilesWithBranch(
	ctx context.Context,
	cfg Config,
	auth Auth,
	files []FileEntry,
	branch *BranchSpec,
	commitCtx CommitContext,
) error {
	if len(files) == 0 {
		return fmt.Errorf("git export: no files to write")
	}

	cfg = cfg.withDefaults()

	req, validated, err := validateExportFiles(cfg, files, branch)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()

	lockKey := cfg.Endpoint
	if lockKey == "" {
		lockKey = req.cloneURL
	}

	fpKey := exportFingerprintKey(lockKey, req.pushBranch, req.objectPath)
	if fingerprintTracker.shouldSkip(fpKey, commitCtx.Checksum) {
		return nil
	}

	if isFileRemote(req.cloneURL) || cfg.Engine == GitEngineCLI {
		var exportErr error
		if err := withRepoExportLock(lockKey, req.pushBranch, func() error {
			exportErr = exportViaCLI(ctx, cfg, auth, req.cloneURL, req.cloneBranch, req.pushBranch, validated, commitCtx)
			if exportErr == nil {
				fingerprintTracker.record(fpKey, commitCtx.Checksum)
			}

			return exportErr
		}); err != nil {
			return ClassifyExportError(err)
		}

		return ClassifyExportError(exportErr)
	}

	var exportErr error
	if err := withRepoExportLock(lockKey, req.pushBranch, func() error {
		exportErr = exportRemote(ctx, cfg, auth, req, validated, commitCtx)
		if exportErr == nil {
			fingerprintTracker.record(fpKey, commitCtx.Checksum)
		}

		return exportErr
	}); err != nil {
		return ClassifyExportError(err)
	}

	return ClassifyExportError(exportErr)
}

func exportRemote(
	ctx context.Context,
	cfg Config,
	auth Auth,
	req exportRequest,
	files []FileEntry,
	commitCtx CommitContext,
) error {
	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	sshCfg := cfg.SSH
	if cfg.TLS.InsecureSkipVerify {
		sshCfg.InsecureSkipVerify = true
	}

	authMethod, err := buildAuthMethodWithForce(req.cloneURL, auth, authType, sshCfg, cfg.ForceBasicAuth)
	if err != nil {
		return err
	}

	workdir, err := prepareMirrorWorkdir(ctx, cfg, auth, req.cloneURL, req.cloneBranch)
	if err != nil {
		return err
	}
	if isFileRemote(req.cloneURL) {
		defer func() { _ = os.RemoveAll(workdir) }()
	}

	repo, emptyRemote, err := openOrWarmMirror(ctx, workdir, req.cloneURL, req.cloneBranch, cfg.CloneDepth, authMethod, cfg)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	if req.pushBranch != req.cloneBranch {
		if checkoutErr := checkoutMirrorBranch(wt, req.pushBranch); checkoutErr != nil {
			return fmt.Errorf("checkout feature branch: %w", checkoutErr)
		}
	} else if checkoutErr := checkoutMirrorBranch(wt, req.pushBranch); checkoutErr != nil {
		return fmt.Errorf("checkout branch: %w", checkoutErr)
	}

	writtenPaths := make([]string, 0, len(files))
	for _, f := range files {
		if mkdirErr := wt.Filesystem.MkdirAll(filepath.Dir(f.Path), 0o750); mkdirErr != nil {
			return fmt.Errorf("mkdir object parent: %w", mkdirErr)
		}

		if writeErr := util.WriteFile(wt.Filesystem, f.Path, f.Data, 0o600); writeErr != nil {
			return fmt.Errorf("write object: %w", writeErr)
		}

		writtenPaths = append(writtenPaths, f.Path)
	}

	if stageErr := stageChanges(wt, writtenPaths, cfg.Prune); stageErr != nil {
		return stageErr
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	if status.IsClean() {
		return nil
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
		return fmt.Errorf("git commit: %w", err)
	}

	return pushCommitted(ctx, repo, cfg, authMethod, req.cloneURL, req.pushBranch, emptyRemote, commit, wt)
}

func stageChanges(wt *git.Worktree, objectPaths []string, prune bool) error {
	if !prune {
		for _, objectPath := range objectPaths {
			if _, addErr := wt.Add(objectPath); addErr != nil {
				return fmt.Errorf("git add: %w", addErr)
			}
		}

		return nil
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		if fileStatus.Worktree == git.Deleted {
			if _, err := wt.Remove(path); err != nil {
				return fmt.Errorf("git remove %q: %w", path, err)
			}

			continue
		}

		if _, err := wt.Add(path); err != nil {
			return fmt.Errorf("git add %q: %w", path, err)
		}
	}

	return nil
}

func resolveBranches(defaultBranch string, spec *BranchSpec) (cloneBranch, pushBranch string) {
	cloneBranch = defaultBranch
	pushBranch = defaultBranch
	if spec == nil {
		return cloneBranch, pushBranch
	}

	if spec.PushBranch != "" {
		pushBranch = spec.PushBranch
	}

	if spec.CloneBranch != "" {
		cloneBranch = spec.CloneBranch
	} else if spec.PushBranch != "" {
		cloneBranch = spec.PushBranch
	}

	return cloneBranch, pushBranch
}

func pushRefSpec(branch string, emptyRemote bool, policy PushPolicy) string {
	refSpecStr := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
	if emptyRemote || policy == PushPolicyForcePush {
		refSpecStr = "+" + refSpecStr
	}

	return refSpecStr
}

func pushCommitted(
	ctx context.Context,
	repo *git.Repository,
	cfg Config,
	authMethod transport.AuthMethod,
	cloneURL, branch string,
	emptyRemote bool,
	commit plumbing.Hash,
	wt *git.Worktree,
) error {
	if refErr := repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(branch), commit,
	)); refErr != nil {
		return fmt.Errorf("set branch ref: %w", refErr)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin: %w", err)
	}

	refSpecStr := pushRefSpec(branch, emptyRemote, cfg.PushPolicy)

	refSpec := config.RefSpec(refSpecStr)
	pushOpts := &git.PushOptions{
		RemoteURL:       cloneURL,
		RefSpecs:        []config.RefSpec{refSpec},
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}

	err = withTransportRetry(ctx, defaultTransportRetry(), func() error {
		pushErr := remote.PushContext(ctx, pushOpts)
		if pushErr == nil || errors.Is(pushErr, git.NoErrAlreadyUpToDate) {
			return nil
		}

		if cfg.PushPolicy == PushPolicyCommit && isNonFastForwardError(pushErr) {
			if syncErr := syncRemoteBeforePush(ctx, repo, wt, authMethod, cloneURL, branch, cfg); syncErr != nil {
				return syncErr
			}

			if head, headErr := repo.Head(); headErr == nil {
				commit = head.Hash()
				if refErr := repo.Storer.SetReference(plumbing.NewHashReference(
					plumbing.NewBranchReferenceName(branch), commit,
				)); refErr != nil {
					return fmt.Errorf("set branch ref after merge: %w", refErr)
				}
			}

			if retryErr := remote.PushContext(ctx, pushOpts); retryErr != nil && !errors.Is(retryErr, git.NoErrAlreadyUpToDate) {
				return retryErr
			}

			return nil
		}

		return pushErr
	})
	if err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func isFileRemote(cloneURL string) bool {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return false
	}

	return u.Scheme == "file"
}

func parseRemote(endpoint string) (cloneURL, branch string, err error) {
	branch = defaultBranch
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", "", fmt.Errorf("parse endpoint: %w", err)
	}

	if frag := strings.TrimPrefix(u.Fragment, "branch="); frag != u.Fragment && frag != "" {
		branch = frag
		u.Fragment = ""
	}

	u.User = nil
	cloneURL = u.String()

	return cloneURL, branch, nil
}

func cloneOrInit(
	ctx context.Context,
	dir, cloneURL, branch string,
	auth transport.AuthMethod,
	cfg Config,
) (*git.Repository, bool, error) {
	cloneOpts := &git.CloneOptions{
		URL:             cloneURL,
		ReferenceName:   plumbing.NewBranchReferenceName(branch),
		SingleBranch:    true,
		Depth:           cfg.CloneDepth,
		Auth:            auth,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}

	var repo *git.Repository
	var cloneErr error

	err := withTransportRetry(ctx, defaultTransportRetry(), func() error {
		repo, cloneErr = git.PlainCloneContext(ctx, dir, false, cloneOpts)
		if cloneErr == nil {
			return nil
		}

		if isEmptyRemote(cloneErr) {
			return nil
		}

		return cloneErr
	})
	if err != nil {
		return nil, false, fmt.Errorf("git clone: %w", err)
	}

	if cloneErr == nil {
		emptyRemote := false
		if _, headErr := repo.Head(); headErr != nil {
			emptyRemote = true
		}

		return repo, emptyRemote, nil
	}

	if !isEmptyRemote(cloneErr) {
		return nil, false, fmt.Errorf("git clone: %w", cloneErr)
	}

	repo, err = git.PlainInit(dir, false)
	if err != nil {
		return nil, false, fmt.Errorf("git init: %w", err)
	}

	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneURL},
	}); err != nil {
		return nil, false, fmt.Errorf("add remote: %w", err)
	}

	return repo, true, nil
}

func isEmptyRemote(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "remote repository is empty") ||
		strings.Contains(msg, "couldn't find remote ref") ||
		strings.Contains(msg, "reference not found")
}

func ExportMemory(payload []byte, objectPath string) (plumbing.Hash, error) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return plumbing.ZeroHash, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	validatedPath, err := validateObjectPath(objectPath)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	objectPath = validatedPath
	if objectPath == "" {
		objectPath = defaultObjectKey
	}

	if err := wt.Filesystem.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return plumbing.ZeroHash, err
	}

	if err := util.WriteFile(wt.Filesystem, objectPath, payload, 0o644); err != nil {
		return plumbing.ZeroHash, err
	}

	if _, err := wt.Add(objectPath); err != nil {
		return plumbing.ZeroHash, err
	}

	return wt.Commit("test", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test", When: time.Now()},
	})
}
