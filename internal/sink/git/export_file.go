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
	"strconv"
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
		if err = runGit(ctx, tmp, "checkout", "-B", pushBranch); err != nil {
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
		if err = runGit(ctx, tmp, "add", "-A"); err != nil {
			return err
		}
	} else if err = runGit(ctx, tmp, "add", gitObjectPath); err != nil {
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
	if err = runGit(ctx, tmp, "-c", "user.name="+cfg.Author.Name, "-c", "user.email="+cfg.Author.Email,
		"commit", "-m", message); err != nil {
		return err
	}

	pushArgs := []string{"push", "-u", "origin", pushBranch}
	if cfg.PushPolicy == PushPolicyForcePush {
		pushArgs = []string{"push", "--force", "-u", "origin", pushBranch}
	}

	if err := runGit(ctx, tmp, pushArgs...); err != nil {
		return err
	}

	return ensureBareHEAD(ctx, cloneURL, pushBranch)
}

func ensureBareHEAD(ctx context.Context, cloneURL, branch string) error {
	u, err := url.Parse(cloneURL)
	if err != nil || u.Scheme != "file" {
		return nil
	}

	bareDir := u.Path
	ref := "refs/heads/" + branch

	//nolint:gosec // G204: bareDir from validated file:// URL
	head := exec.CommandContext(ctx, "git", "--git-dir="+bareDir, "symbolic-ref", "-q", "HEAD")
	if head.Run() == nil {
		return nil
	}

	//nolint:gosec // G204: bareDir from validated file:// URL
	setHead := exec.CommandContext(ctx, "git", "--git-dir="+bareDir, "symbolic-ref", "HEAD", ref)
	if out, err := setHead.CombinedOutput(); err != nil {
		return fmt.Errorf("git symbolic-ref HEAD %s: %s: %w", ref, strings.TrimSpace(string(out)), err)
	}

	return nil
}

func cloneOrInitCLI(ctx context.Context, dir, cloneURL, branch string, depth int) error {
	cloneArgs := []string{"clone", "--branch", branch, "--single-branch"}
	if depth > 0 {
		cloneArgs = append(cloneArgs, "--depth", strconv.Itoa(depth))
	}

	cloneArgs = append(cloneArgs, cloneURL, dir)

	//nolint:gosec // G204: dir from MkdirTemp; branch/URL validated before invocation
	clone := exec.CommandContext(ctx, "git", cloneArgs...)
	if out, err := clone.CombinedOutput(); err == nil {
		return nil
	} else if !isCLIEmptyRemote(string(out), err) {
		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if rmErr := os.RemoveAll(dir); rmErr != nil {
		return fmt.Errorf("reset workdir: %w", rmErr)
	}

	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil { //nolint:gosec // G301: temp dir
		return fmt.Errorf("mkdir workdir: %w", mkErr)
	}

	if err := runGit(ctx, dir, "init"); err != nil {
		return err
	}

	if err := runGit(ctx, dir, "checkout", "-B", branch); err != nil {
		return err
	}

	return runGit(ctx, dir, "remote", "add", "origin", cloneURL)
}

func gitStatusClean(ctx context.Context, dir string) (bool, error) {
	//nolint:gosec // G204: dir from MkdirTemp
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return len(strings.TrimSpace(string(out))) == 0, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	//nolint:gosec // G204: dir from MkdirTemp; args validated at export entry
	cmd := exec.CommandContext(ctx, "git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}

	return nil
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
