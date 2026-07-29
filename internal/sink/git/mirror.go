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
	"syscall"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	envMirrorDir   = "KOLLECT_GIT_MIRROR_DIR"
	mirrorRootPerm = 0o700
)

// errMirrorRootInsecure means a mirror root directory already existed but
// wasn't safe to reuse. A predictable path under a shared temp/emptyDir
// parent can be pre-created by another tenant before kollect starts; treating
// whatever is already there as trustworthy would let that tenant plant a
// symlink or a fake "warm" mirror for kollect to read from or write into.
var errMirrorRootInsecure = errors.New("mirror root directory is not safe to reuse")

func defaultMirrorRoot() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "kollect", "git-mirrors")
	}

	return filepath.Join(os.TempDir(), "kollect-git-mirrors")
}

func mirrorRootDir() (string, error) {
	dir := strings.TrimSpace(os.Getenv(envMirrorDir))
	if dir == "" {
		dir = defaultMirrorRoot()
	}

	return ensureSecureMirrorRoot(dir)
}

// ensureSecureMirrorRoot creates dir (mode mirrorRootPerm) if it doesn't
// exist yet. If it does exist, it refuses to adopt it unless it is a real
// directory, owned by the current process, with no group/other write bit.
// dir is operator config (KOLLECT_GIT_MIRROR_DIR) or our own computed
// default, not attacker input -- validating it is this function's job.
func ensureSecureMirrorRoot(dir string) (string, error) {
	info, err := os.Lstat(dir) //nolint:gosec // G703: dir is operator/default config, validated below
	if errors.Is(err, os.ErrNotExist) {
		if mkErr := os.MkdirAll(dir, mirrorRootPerm); mkErr != nil { //nolint:gosec // G703: see above
			return "", fmt.Errorf("mirror root: %w", mkErr)
		}

		return dir, nil
	}
	if err != nil {
		return "", fmt.Errorf("mirror root: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: %s is a symlink", errMirrorRootInsecure, dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %s is not a directory", errMirrorRootInsecure, dir)
	}
	// Group/other WRITE is the actual hazard: it lets another local tenant
	// plant or swap content under a path kollect will then trust. Wider but
	// non-writable read/exec bits (e.g. 0755 from some emptyDir defaults)
	// aren't exploitable the same way and are left alone.
	if info.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("%w: %s is group/other-writable (%04o)",
			errMirrorRootInsecure, dir, info.Mode().Perm())
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return "", fmt.Errorf("%w: %s is owned by uid %d, not the current user",
			errMirrorRootInsecure, dir, stat.Uid)
	}

	return dir, nil
}

func mirrorDirFor(cloneURL, branch string) (string, error) {
	root, err := mirrorRootDir()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(cloneURL + "\x00" + branch))
	dir := filepath.Join(root, hex.EncodeToString(sum[:16]))
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
