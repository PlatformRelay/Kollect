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

func gitClone(ctx context.Context, workdir, cloneURL, branch string, depth int) (cloned bool, err error) {
	if err := ValidateGitRef(branch); err != nil {
		return false, fmt.Errorf("git export: invalid branch: %w", err)
	}

	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	var cmd *exec.Cmd
	if depth > 0 {
		cmd = exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--depth", strconv.Itoa(depth), "--", safeURL, workdir)
	} else {
		cmd = exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--", safeURL, workdir)
	}

	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}

	if isCLIEmptyRemote(string(out), err) {
		return false, nil
	}

	return false, fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
}

func gitInit(ctx context.Context, workdir string) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "init")
	return runGitOutput(cmd, "init")
}

func gitCheckoutNewBranch(ctx context.Context, workdir, branch string) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "checkout", "-B", branch)
	return runGitOutput(cmd, "checkout -B "+branch)
}

func gitRemoteAddOrigin(ctx context.Context, workdir, cloneURL string) error {
	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "remote", "add", "origin", safeURL)
	return runGitOutput(cmd, "remote add origin")
}

func gitAddPath(ctx context.Context, workdir, objectPath string) error {
	validatedPath, err := validateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "add", validatedPath)
	return runGitOutput(cmd, "add "+validatedPath)
}

func gitAddAll(ctx context.Context, workdir string) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "add", "-A")
	return runGitOutput(cmd, "add -A")
}

func gitCommit(ctx context.Context, workdir, authorName, authorEmail, message string) error {
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

	cmd := exec.CommandContext(ctx, "git", "-C", workdir,
		"-c", "user.name="+authorName,
		"-c", "user.email="+authorEmail,
		"commit", "-m", message,
	)
	return runGitOutput(cmd, "commit")
}

func gitPushOrigin(ctx context.Context, workdir string, force bool, branch string) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	var cmd *exec.Cmd
	if force {
		cmd = exec.CommandContext(ctx, "git", "-C", workdir, "push", "--force", "-u", "origin", branch)
	} else {
		cmd = exec.CommandContext(ctx, "git", "-C", workdir, "push", "-u", "origin", branch)
	}

	return runGitOutput(cmd, "push")
}

func gitStatusPorcelain(ctx context.Context, workdir string) (string, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", fmt.Errorf("git export: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workdir, "status", "--porcelain")
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
