// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// RemoteBranchExists reports whether branch exists on the configured remote,
// without a local worktree (remote list). Cleanup uses it to find a deletion
// commit an earlier attempt pushed but never opened a merge request for; an
// error must be retried, not treated as "absent".
func RemoteBranchExists(ctx context.Context, cfg Config, auth Auth, branch string) (bool, error) {
	cfg = cfg.withDefaults()

	cloneURL, _, err := parseRemote(cfg.Endpoint)
	if err != nil {
		return false, err
	}
	if err = validateCloneURL(cloneURL); err != nil {
		return false, fmt.Errorf("git remote lookup: %w", err)
	}
	if err = ValidateGitRef(branch); err != nil {
		return false, fmt.Errorf("git remote lookup: invalid branch: %w", err)
	}

	authType := auth.AuthType
	if authType == "" {
		authType = cfg.AuthType
	}

	authMethod, _, err := guardedGoGitAuth(ctx, exportRequest{cloneURL: cloneURL}, auth, authType, cfg.effectiveSSHConfig(), cfg.ForceBasicAuth)
	if err != nil {
		return false, err
	}

	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneURL},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	})
	if err != nil {
		return false, fmt.Errorf("git remote lookup: %w", err)
	}

	target := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == target {
			return true, nil
		}
	}

	return false, nil
}
