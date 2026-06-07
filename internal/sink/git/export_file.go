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

func exportViaCLI(
	ctx context.Context,
	cfg Config,
	auth Auth,
	cloneURL, cloneBranch, pushBranch string,
	files []FileEntry,
	commitCtx CommitContext,
) error {
	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	cli, err := newCLIEnv(cfg, auth, authType)
	if err != nil {
		return err
	}
	defer cli.cleanup()

	workdir, err := prepareMirrorWorkdir(ctx, cfg, auth, cloneURL, cloneBranch)
	if err != nil {
		return err
	}
	if isFileRemote(cloneURL) {
		defer func() { _ = os.RemoveAll(workdir) }()
	}

	cloneURLForCLI := cloneURL
	if creds := auth.embedInURL(cloneURL); creds != "" && !cfg.ForceBasicAuth {
		cloneURLForCLI = creds
	}

	if err = prepareCLIWorkdir(ctx, workdir, cloneURLForCLI, cloneBranch, pushBranch, cfg, cli); err != nil {
		return err
	}

	gitObjectPaths := make([]string, 0, len(files))
	for _, f := range files {
		target, gitObjectPath, pathErr := objectPathInWorkdir(workdir, f.Path)
		if pathErr != nil {
			return fmt.Errorf("git export: %w", pathErr)
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750); mkdirErr != nil { //nolint:gosec // G301: temp dir
			return fmt.Errorf("mkdir object parent: %w", mkdirErr)
		}

		//nolint:gosec // G306: temp file in validated workdir
		if writeErr := os.WriteFile(target, f.Data, 0o600); writeErr != nil {
			return fmt.Errorf("write object: %w", writeErr)
		}

		gitObjectPaths = append(gitObjectPaths, gitObjectPath)
	}

	if cfg.Prune {
		if err = gitAddAll(ctx, workdir, cli); err != nil {
			return err
		}
	} else {
		for _, gitObjectPath := range gitObjectPaths {
			if err = gitAddPath(ctx, workdir, gitObjectPath, cli); err != nil {
				return err
			}
		}
	}

	clean, statusErr := gitStatusClean(ctx, workdir, cli)
	if statusErr != nil {
		return statusErr
	}
	if clean {
		return nil
	}

	commitText := renderCommit(cfg, commitCtx)
	if err = gitCommit(ctx, workdir, cfg.Author.Name, cfg.Author.Email, commitText, cli); err != nil {
		return err
	}

	forcePush := cfg.PushPolicy == PushPolicyForcePush
	if err := gitPushOriginWithRecovery(ctx, workdir, forcePush, pushBranch, cfg, cli); err != nil {
		return err
	}

	return ensureBareHEAD(ctx, cloneURL, pushBranch, cli)
}

func prepareCLIWorkdir(
	ctx context.Context,
	workdir, cloneURL, cloneBranch, pushBranch string,
	cfg Config,
	cli *cliEnv,
) error {
	if mirrorWarm(workdir) {
		if err := gitFetchShallow(ctx, workdir, cloneBranch, cfg.CloneDepth, cli); err != nil {
			return err
		}

		return gitCheckoutNewBranch(ctx, workdir, pushBranch, cli)
	}

	if err := cloneOrInitCLI(ctx, workdir, cloneURL, cloneBranch, cfg.CloneDepth, cli); err != nil {
		return err
	}

	if pushBranch == cloneBranch {
		return nil
	}

	return gitCheckoutNewBranch(ctx, workdir, pushBranch, cli)
}

func gitPushOriginWithRecovery(
	ctx context.Context,
	workdir string,
	force bool,
	branch string,
	cfg Config,
	cli *cliEnv,
) error {
	err := gitPushOrigin(ctx, workdir, force, branch, cli)
	if err == nil || force || cfg.PushPolicy != PushPolicyCommit {
		return err
	}

	if !isNonFastForwardError(err) {
		return err
	}

	if pullErr := gitPullRebase(ctx, workdir, branch, cli); pullErr != nil {
		return pullErr
	}

	return gitPushOrigin(ctx, workdir, false, branch, cli)
}

func ensureBareHEAD(ctx context.Context, cloneURL, branch string, cli *cliEnv) error {
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
	applyCLIEnv(head, cli)
	if head.Run() == nil {
		return nil
	}

	//nolint:gosec // G204: bareDir from parseFileGitBarePath; ref from validated branch
	setHead := exec.CommandContext(ctx, "git", "--git-dir", bareDir, "symbolic-ref", "HEAD", ref)
	applyCLIEnv(setHead, cli)
	if out, err := setHead.CombinedOutput(); err != nil {
		return fmt.Errorf("git symbolic-ref HEAD %s: %s: %w", ref, strings.TrimSpace(string(out)), err)
	}

	return nil
}

func cloneOrInitCLI(ctx context.Context, dir, cloneURL, branch string, depth int, cli *cliEnv) error {
	cloned, err := gitClone(ctx, dir, cloneURL, branch, depth, cli)
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

	if err := gitInit(ctx, dir, cli); err != nil {
		return err
	}

	if err := gitCheckoutNewBranch(ctx, dir, branch, cli); err != nil {
		return err
	}

	return gitRemoteAddOrigin(ctx, dir, cloneURL, cli)
}

func gitStatusClean(ctx context.Context, dir string, cli *cliEnv) (bool, error) {
	out, err := gitStatusPorcelain(ctx, dir, cli)
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
