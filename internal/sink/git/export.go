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
	commitCtx := CommitContextFromObjectPath(objectPath, cfg.Cluster)
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

	cfg = cfg.withDefaults()

	req, err := validateExportRequest(cfg, objectPath, branch)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()

	if isFileRemote(req.cloneURL) {
		return exportFileRemote(ctx, cfg, req.cloneURL, req.cloneBranch, req.pushBranch, payload, req.objectPath, commitCtx)
	}

	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	authMethod, err := buildAuthMethod(req.cloneURL, auth, authType)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "kollect-git-export-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	repo, emptyRemote, err := cloneOrInit(ctx, tmp, req.cloneURL, req.cloneBranch, authMethod, cfg)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	if req.pushBranch != req.cloneBranch {
		if checkoutErr := wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(req.pushBranch),
			Create: true,
		}); checkoutErr != nil {
			return fmt.Errorf("checkout feature branch: %w", checkoutErr)
		}
	}

	if mkdirErr := wt.Filesystem.MkdirAll(filepath.Dir(req.objectPath), 0o750); mkdirErr != nil {
		return fmt.Errorf("mkdir object parent: %w", mkdirErr)
	}

	if writeErr := util.WriteFile(wt.Filesystem, req.objectPath, payload, 0o600); writeErr != nil {
		return fmt.Errorf("write object: %w", writeErr)
	}

	if stageErr := stageChanges(wt, req.objectPath, cfg.Prune); stageErr != nil {
		return stageErr
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	if status.IsClean() {
		return nil
	}

	message := renderCommitMessage(cfg.CommitMessage, commitCtx)
	commit, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  cfg.Author.Name,
			Email: cfg.Author.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return pushCommitted(ctx, repo, cfg, authMethod, req.cloneURL, req.pushBranch, emptyRemote, commit)
}

func stageChanges(wt *git.Worktree, objectPath string, prune bool) error {
	if !prune {
		if _, addErr := wt.Add(objectPath); addErr != nil {
			return fmt.Errorf("git add: %w", addErr)
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
	if err := remote.PushContext(ctx, &git.PushOptions{
		RemoteURL:       cloneURL,
		RefSpecs:        []config.RefSpec{refSpec},
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
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

	repo, err := git.PlainCloneContext(ctx, dir, false, cloneOpts)
	if err == nil {
		emptyRemote := false
		if _, headErr := repo.Head(); headErr != nil {
			emptyRemote = true
		}

		return repo, emptyRemote, nil
	}

	if !isEmptyRemote(err) {
		return nil, false, fmt.Errorf("git clone: %w", err)
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
