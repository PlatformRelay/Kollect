// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func gitInWorkdir(ctx context.Context, workdir string, cli *cliEnv, args ...string) *exec.Cmd {
	argv := make([]string, 0, 4+len(args))
	argv = append(argv, "git")
	if cli != nil {
		argv = append(argv, cli.prependGitArgs("-C", workdir)...)
	} else {
		argv = append(argv, "-C", workdir)
	}
	argv = append(argv, args...)
	//nolint:gosec // G204: workdir validated by validateGitWorkdir before call
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	applyCLIEnv(cmd, cli)

	return cmd
}

func gitCloneCmd(ctx context.Context, cli *cliEnv, args ...string) *exec.Cmd {
	cloneArgs := args
	if cli != nil {
		cloneArgs = cli.prependGitArgs(args...)
	}

	argv := append([]string{"git", "clone"}, cloneArgs...)
	//nolint:gosec // G204: cloneURL, workdir, and branch validated before call
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	applyCLIEnv(cmd, cli)

	return cmd
}

func gitClone(ctx context.Context, workdir, cloneURL, branch string, depth int, cli *cliEnv) (cloned bool, err error) {
	if validateErr := ValidateGitRef(branch); validateErr != nil {
		return false, fmt.Errorf("git export: invalid branch: %w", validateErr)
	}

	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	var cloneArgs []string
	if depth > 0 {
		cloneArgs = []string{"--branch", branch, "--single-branch", "--depth", strconv.Itoa(depth), "--", safeURL, workdir}
	} else {
		cloneArgs = []string{"--branch", branch, "--single-branch", "--", safeURL, workdir}
	}

	var out []byte
	retryErr := withTransportRetry(ctx, defaultTransportRetry(), func() error {
		cmd := gitCloneCmd(ctx, cli, cloneArgs...)
		out, err = cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		if isCLIEmptyRemote(string(out), err) {
			return nil
		}

		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
	})
	if retryErr != nil {
		return false, retryErr
	}

	if err == nil {
		return true, nil
	}

	if isCLIEmptyRemote(string(out), err) {
		return false, nil
	}

	return false, fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
}

func gitInit(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "init")
	return runGitOutput(cmd, "init")
}

func gitCheckoutNewBranch(ctx context.Context, workdir, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "checkout", "-B", branch)
	return runGitOutput(cmd, "checkout -B "+branch)
}

func gitRemoteAddOrigin(ctx context.Context, workdir, cloneURL string, cli *cliEnv) error {
	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "remote", "add", "origin", safeURL)
	return runGitOutput(cmd, "remote add origin")
}

func gitAddPath(ctx context.Context, workdir, objectPath string, cli *cliEnv) error {
	validatedPath, err := validateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "add", validatedPath)
	return runGitOutput(cmd, "add "+validatedPath)
}

func gitAddAll(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "add", "-A")
	return runGitOutput(cmd, "add -A")
}

func gitCommit(ctx context.Context, workdir, authorName, authorEmail, message string, cli *cliEnv) error {
	if err := validateGitConfigValue(authorName); err != nil {
		return fmt.Errorf("git export: invalid author name: %w", err)
	}

	if err := validateGitConfigValue(authorEmail); err != nil {
		return fmt.Errorf("git export: invalid author email: %w", err)
	}

	if err := validateGitCommitMessage(message); err != nil {
		return fmt.Errorf("git export: invalid commit message: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli,
		"-c", "user.name="+authorName,
		"-c", "user.email="+authorEmail,
		"commit", "-m", message,
	)
	return runGitOutput(cmd, "commit")
}

func gitPushOrigin(ctx context.Context, workdir string, force bool, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	var cmd *exec.Cmd
	if force {
		cmd = gitInWorkdir(ctx, workdir, cli, "push", "--force", "-u", "origin", branch)
	} else {
		cmd = gitInWorkdir(ctx, workdir, cli, "push", "-u", "origin", branch)
	}

	return runGitOutput(cmd, "push")
}

func gitPullRebase(ctx context.Context, workdir string, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "pull", "--rebase", "origin", branch)
	return runGitOutput(cmd, "pull --rebase")
}

func gitStatusPorcelain(ctx context.Context, workdir string, cli *cliEnv) (string, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return string(out), nil
}

func runGitOutput(cmd *exec.Cmd, label string) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", label, strings.TrimSpace(string(out)), err)
	}

	return nil
}
