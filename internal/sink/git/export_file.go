// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func exportFileRemote(
	ctx context.Context,
	cfg Config,
	cloneURL, cloneBranch, pushBranch string,
	payload []byte,
	objectPath string,
	commitCtx CommitContext,
) error {
	if _, err := validateObjectPath(objectPath); err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	tmp, err := os.MkdirTemp("", "kollect-git-export-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err = cloneOrInitCLI(ctx, tmp, cloneURL, cloneBranch, cfg.CloneDepth); err != nil {
		return err
	}

	if pushBranch != cloneBranch {
		if err = gitCheckoutNewBranch(ctx, tmp, pushBranch); err != nil {
			return err
		}
	}

	target, gitObjectPath, err := objectPathInWorkdir(tmp, objectPath)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750); mkdirErr != nil { //nolint:gosec // G301: temp dir
		return fmt.Errorf("mkdir object parent: %w", mkdirErr)
	}

	//nolint:gosec // G306: temp file in validated workdir
	if writeErr := os.WriteFile(target, payload, 0o600); writeErr != nil {
		return fmt.Errorf("write object: %w", writeErr)
	}

	if cfg.Prune {
		if err = gitAddAll(ctx, tmp); err != nil {
			return err
		}
	} else if err = gitAddPath(ctx, tmp, gitObjectPath); err != nil {
		return err
	}

	clean, statusErr := gitStatusClean(ctx, tmp)
	if statusErr != nil {
		return statusErr
	}
	if clean {
		return nil
	}

	message := renderCommitMessage(cfg.CommitMessage, commitCtx)
	if err = gitCommit(ctx, tmp, cfg.Author.Name, cfg.Author.Email, message); err != nil {
		return err
	}

	forcePush := cfg.PushPolicy == PushPolicyForcePush
	if err := gitPushOrigin(ctx, tmp, forcePush, pushBranch); err != nil {
		return err
	}

	return ensureBareHEAD(ctx, cloneURL, pushBranch)
}

func ensureBareHEAD(ctx context.Context, cloneURL, branch string) error {
	u, err := url.Parse(cloneURL)
	if err != nil || u.Scheme != schemeFile {
		return nil
	}

	if err = ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	bareDir, err := parseFileGitBarePath(cloneURL)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	ref := "refs/heads/" + branch

	//nolint:gosec // G204: bareDir from parseFileGitBarePath; branch from ValidateGitRef
	head := exec.CommandContext(ctx, "git", "--git-dir", bareDir, "symbolic-ref", "-q", "HEAD")
	if head.Run() == nil {
		return nil
	}

	//nolint:gosec // G204: bareDir from parseFileGitBarePath; ref from validated branch
	setHead := exec.CommandContext(ctx, "git", "--git-dir", bareDir, "symbolic-ref", "HEAD", ref)
	if out, err := setHead.CombinedOutput(); err != nil {
		return fmt.Errorf("git symbolic-ref HEAD %s: %s: %w", ref, strings.TrimSpace(string(out)), err)
	}

	return nil
}

func cloneOrInitCLI(ctx context.Context, dir, cloneURL, branch string, depth int) error {
	cloned, err := gitClone(ctx, dir, cloneURL, branch, depth)
	if err != nil {
		return err
	}

	if cloned {
		return nil
	}

	if rmErr := os.RemoveAll(dir); rmErr != nil {
		return fmt.Errorf("reset workdir: %w", rmErr)
	}

	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil { //nolint:gosec // G301: temp dir
		return fmt.Errorf("mkdir workdir: %w", mkErr)
	}

	if err := gitInit(ctx, dir); err != nil {
		return err
	}

	if err := gitCheckoutNewBranch(ctx, dir, branch); err != nil {
		return err
	}

	return gitRemoteAddOrigin(ctx, dir, cloneURL)
}

func gitStatusClean(ctx context.Context, dir string) (bool, error) {
	out, err := gitStatusPorcelain(ctx, dir)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(out)) == 0, nil
}

func isCLIEmptyRemote(output string, err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(output + " " + err.Error())

	return strings.Contains(msg, "remote repository is empty") ||
		strings.Contains(msg, "couldn't find remote ref") ||
		strings.Contains(msg, "reference not found") ||
		strings.Contains(msg, "repository not found") ||
		strings.Contains(msg, "remote branch") && strings.Contains(msg, "not found")
}
