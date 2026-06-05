// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func exportFileRemote(
	ctx context.Context,
	cloneURL, branch string,
	payload []byte,
	objectPath string,
) error {
	tmp, err := os.MkdirTemp("", "kollect-git-export-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err := cloneOrInitCLI(ctx, tmp, cloneURL, branch); err != nil {
		return err
	}

	target := filepath.Join(tmp, filepath.FromSlash(objectPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750); mkdirErr != nil { //nolint:gosec // G301: temp dir
		return fmt.Errorf("mkdir object parent: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(target, payload, 0o600); writeErr != nil { //nolint:gosec // G306: temp file
		return fmt.Errorf("write object: %w", writeErr)
	}

	if err := runGit(ctx, tmp, "add", objectPath); err != nil {
		return err
	}

	if clean, err := gitStatusClean(ctx, tmp); err != nil {
		return err
	} else if clean {
		return nil
	}

	if err := runGit(ctx, tmp, "-c", "user.name=kollect", "-c", "user.email=kollect@kollect.dev",
		"commit", "-m", "kollect: export inventory"); err != nil {
		return err
	}

	ref := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)

	return runGit(ctx, tmp, "push", "--force", "origin", ref)
}

func cloneOrInitCLI(ctx context.Context, dir, cloneURL, branch string) error {
	//nolint:gosec // G204: dir from MkdirTemp
	clone := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", cloneURL, dir)
	if out, err := clone.CombinedOutput(); err == nil {
		return nil
	} else if !isCLIEmptyRemote(string(out), err) {
		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
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
	//nolint:gosec // G204: dir from MkdirTemp
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
