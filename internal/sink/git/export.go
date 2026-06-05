// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	defaultBranch    = "main"
	defaultObjectKey = "inventory/latest.json"
	exportTimeout    = 2 * time.Minute
)

// Auth holds optional credentials for git push.
type Auth struct {
	Username string
	Password string
	Token    string
}

// Export clones (or initializes), writes payload at objectPath, commits, and pushes to the remote.
func Export(ctx context.Context, cfg Config, auth Auth, payload []byte, objectPath string) error {
	if len(payload) == 0 {
		return fmt.Errorf("git export: empty payload")
	}

	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		objectPath = defaultObjectKey
	}

	cloneURL, branch, err := parseRemote(cfg.Endpoint)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()

	authMethod, err := basicAuth(cloneURL, auth)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "kollect-git-export-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	repo, err := cloneOrInit(ctx, tmp, cloneURL, branch, authMethod, cfg)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	target := filepath.Join(tmp, filepath.FromSlash(objectPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o750); mkdirErr != nil { //nolint:gosec // G301: temp dir
		return fmt.Errorf("mkdir object parent: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(target, payload, 0o600); writeErr != nil { //nolint:gosec // G306: temp file
		return fmt.Errorf("write object: %w", writeErr)
	}

	if _, addErr := wt.Add(objectPath); addErr != nil {
		return fmt.Errorf("git add: %w", addErr)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	if status.IsClean() {
		return nil
	}

	commit, err := wt.Commit("kollect: export inventory", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "kollect",
			Email: "kollect@kollect.dev",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if refErr := repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(branch), commit,
	)); refErr != nil {
		return fmt.Errorf("set branch ref: %w", refErr)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin: %w", err)
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	if err := remote.PushContext(ctx, &git.PushOptions{
		RemoteURL:       cloneURL,
		RefSpecs:        []config.RefSpec{refSpec},
		Auth:            authMethod,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func parseRemote(endpoint string) (cloneURL, branch string, err error) {
	branch = defaultBranch
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", "", fmt.Errorf("parse endpoint: %w", err)
	}

	if frag := strings.TrimPrefix(u.Fragment, "branch="); frag != u.Fragment && frag != "" {
		branch = frag
		u.Fragment = ""
	}

	u.User = nil
	cloneURL = u.String()

	return cloneURL, branch, nil
}

func basicAuth(cloneURL string, auth Auth) (transport.AuthMethod, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		if auth.Username == "" && auth.Token == "" && auth.Password == "" {
			return nil, nil
		}

		user := auth.Username
		if user == "" {
			user = "git"
		}

		pass := auth.Token
		if pass == "" {
			pass = auth.Password
		}

		return &githttp.BasicAuth{Username: user, Password: pass}, nil
	case "file":
		return nil, nil
	case "ssh":
		return nil, fmt.Errorf("ssh git export is not supported yet; use https")
	default:
		return nil, fmt.Errorf("unsupported git URL scheme %q", u.Scheme)
	}
}

func cloneOrInit(
	ctx context.Context,
	dir, cloneURL, branch string,
	auth transport.AuthMethod,
	cfg Config,
) (*git.Repository, error) {
	cloneOpts := &git.CloneOptions{
		URL:             cloneURL,
		ReferenceName:   plumbing.NewBranchReferenceName(branch),
		SingleBranch:    true,
		Depth:           1,
		Auth:            auth,
		InsecureSkipTLS: cfg.TLS.InsecureSkipVerify,
		CABundle:        cfg.CABundle,
	}

	repo, err := git.PlainCloneContext(ctx, dir, false, cloneOpts)
	if err == nil {
		return repo, nil
	}

	if !isEmptyRemote(err) {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	repo, err = git.PlainInit(dir, false)
	if err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}

	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneURL},
	}); err != nil {
		return nil, fmt.Errorf("add remote: %w", err)
	}

	return repo, nil
}

func isEmptyRemote(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "remote repository is empty") ||
		strings.Contains(msg, "couldn't find remote ref") ||
		strings.Contains(msg, "reference not found")
}

// ExportMemory performs an in-memory commit (for unit tests without a remote).
func ExportMemory(payload []byte, objectPath string) (plumbing.Hash, error) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return plumbing.ZeroHash, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		objectPath = defaultObjectKey
	}

	if err := wt.Filesystem.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return plumbing.ZeroHash, err
	}

	if err := util.WriteFile(wt.Filesystem, objectPath, payload, 0o644); err != nil {
		return plumbing.ZeroHash, err
	}

	if _, err := wt.Add(objectPath); err != nil {
		return plumbing.ZeroHash, err
	}

	return wt.Commit("test", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test", When: time.Now()},
	})
}
