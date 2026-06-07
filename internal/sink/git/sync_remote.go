// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func syncRemoteBeforePush(
	ctx context.Context,
	repo *git.Repository,
	wt *git.Worktree,
	auth transport.AuthMethod,
	cloneURL, branch string,
	cfg Config,
) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin: %w", err)
	}

	refSpec := config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", branch, branch))
	fetchOpts := &git.FetchOptions{
		RemoteURL:       cloneURL,
		RefSpecs:        []config.RefSpec{refSpec},
		Auth:            auth,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}

	if fetchErr := withTransportRetry(ctx, defaultTransportRetry(), func() error {
		return remote.FetchContext(ctx, fetchOpts)
	}); fetchErr != nil && fetchErr != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git fetch before push: %w", fetchErr)
	}

	if pullErr := wt.PullContext(ctx, &git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Auth:          auth,
	}); pullErr != nil && pullErr != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git pull merge before push: %w", pullErr)
	}

	return nil
}
