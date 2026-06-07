// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const envMirrorDir = "KOLLECT_GIT_MIRROR_DIR"

func mirrorRootDir() string {
	if dir := strings.TrimSpace(os.Getenv(envMirrorDir)); dir != "" {
		return dir
	}

	return filepath.Join(os.TempDir(), "kollect-git-mirrors")
}

func mirrorDirFor(cloneURL, branch string) (string, error) {
	sum := sha256.Sum256([]byte(cloneURL + "\x00" + branch))
	dir := filepath.Join(mirrorRootDir(), hex.EncodeToString(sum[:16]))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("mirror dir: %w", err)
	}

	return dir, nil
}

func mirrorWarm(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func openOrWarmMirror(
	ctx context.Context,
	dir, cloneURL, branch string,
	depth int,
	auth transport.AuthMethod,
	cfg Config,
) (*git.Repository, bool, error) {
	if !mirrorWarm(dir) {
		return cloneOrInit(ctx, dir, cloneURL, branch, auth, cfg)
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, false, fmt.Errorf("open mirror: %w", err)
	}

	fetchErr := withTransportRetry(ctx, defaultTransportRetry(), func() error {
		return repo.FetchContext(ctx, &git.FetchOptions{
			RemoteName: "origin",
			RefSpecs: []config.RefSpec{
				config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", branch, branch)),
			},
			Depth:           depth,
			Auth:            auth,
			InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
			CABundle:        cfg.CABundle,
		})
	})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		return nil, false, fmt.Errorf("mirror fetch: %w", fetchErr)
	}

	emptyRemote := false
	if _, headErr := repo.Head(); headErr != nil {
		emptyRemote = true
	}

	return repo, emptyRemote, nil
}

func prepareMirrorWorkdir(_ context.Context, _ Config, _ Auth, cloneURL, cloneBranch string) (string, error) {
	if isFileRemote(cloneURL) {
		tmp, err := os.MkdirTemp("", "kollect-git-export-*")
		if err != nil {
			return "", fmt.Errorf("create workdir: %w", err)
		}

		return tmp, nil
	}

	return mirrorDirFor(cloneURL, cloneBranch)
}

func checkoutMirrorBranch(wt *git.Worktree, branch string) error {
	ref := plumbing.NewBranchReferenceName(branch)
	if err := wt.Checkout(&git.CheckoutOptions{Branch: ref, Create: true}); err == nil {
		return nil
	}

	return wt.Checkout(&git.CheckoutOptions{Branch: ref})
}
