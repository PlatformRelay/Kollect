// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konih/kollect/internal/sink/pathvalidate"
)

var safeGitRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func validateObjectPath(objectPath string) (string, error) {
	return pathvalidate.ValidateRelativeObjectPath(objectPath)
}

func objectPathInWorkdir(workdir, objectPath string) (absPath string, relPath string, err error) {
	relPath, err = validateObjectPath(objectPath)
	if err != nil {
		return "", "", err
	}

	absPath = filepath.Join(workdir, filepath.FromSlash(relPath))
	resolved, err := filepath.Abs(absPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve object path: %w", err)
	}

	workdirAbs, err := filepath.Abs(workdir)
	if err != nil {
		return "", "", fmt.Errorf("resolve workdir: %w", err)
	}

	rel, err := filepath.Rel(workdirAbs, resolved)
	if err != nil {
		return "", "", fmt.Errorf("object path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("object path escapes workdir")
	}

	return resolved, relPath, nil
}

func ValidateGitRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("empty git ref")
	}

	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("git ref %q must not start with '-'", ref)
	}

	if ref == "." || ref == ".." || strings.Contains(ref, "..") {
		return fmt.Errorf("git ref %q contains invalid '..'", ref)
	}

	if strings.HasPrefix(ref, ".") {
		return fmt.Errorf("git ref %q must not start with '.'", ref)
	}

	if strings.HasSuffix(ref, ".") || strings.HasSuffix(ref, ".lock") {
		return fmt.Errorf("git ref %q has invalid suffix", ref)
	}

	if ref == "@" || strings.Contains(ref, "@{") {
		return fmt.Errorf("git ref %q contains invalid '@'", ref)
	}

	if strings.HasPrefix(ref, "refs/") {
		return fmt.Errorf("git ref %q must be a short branch name", ref)
	}

	if !safeGitRefPattern.MatchString(ref) {
		return fmt.Errorf("git ref %q contains unsupported characters", ref)
	}

	return nil
}

type exportRequest struct {
	cloneURL    string
	cloneBranch string
	pushBranch  string
	objectPath  string
}

func validateExportRequest(cfg Config, objectPath string, branch *BranchSpec) (exportRequest, error) {
	validatedPath, err := validateObjectPath(objectPath)
	if err != nil {
		return exportRequest{}, fmt.Errorf("git export: %w", err)
	}

	objectPath = validatedPath
	if objectPath == "" {
		objectPath = defaultObjectKey
	}

	cloneURL, defaultBranch, err := parseRemote(cfg.Endpoint)
	if err != nil {
		return exportRequest{}, err
	}

	if err = validateCloneURL(cloneURL); err != nil {
		return exportRequest{}, fmt.Errorf("git export: %w", err)
	}

	cloneBranch, pushBranch := resolveBranches(cfg.EffectiveBranch(defaultBranch), branch)

	if err = ValidateGitRef(cloneBranch); err != nil {
		return exportRequest{}, fmt.Errorf("git export: invalid clone branch: %w", err)
	}

	if err = ValidateGitRef(pushBranch); err != nil {
		return exportRequest{}, fmt.Errorf("git export: invalid push branch: %w", err)
	}

	return exportRequest{
		cloneURL:    cloneURL,
		cloneBranch: cloneBranch,
		pushBranch:  pushBranch,
		objectPath:  objectPath,
	}, nil
}

func validateCloneURL(cloneURL string) error {
	cloneURL = strings.TrimSpace(cloneURL)
	if cloneURL == "" {
		return fmt.Errorf("empty clone URL")
	}

	if strings.HasPrefix(cloneURL, "-") {
		return fmt.Errorf("clone URL must not start with '-'")
	}

	u, err := url.Parse(cloneURL)
	if err != nil {
		return fmt.Errorf("invalid clone URL: %w", err)
	}

	switch u.Scheme {
	case schemeFile:
		if _, err = parseFileGitBarePath(cloneURL); err != nil {
			return err
		}
	case schemeHTTP, schemeHTTPS, schemeSSH:
		return nil
	default:
		return fmt.Errorf("unsupported clone URL scheme %q", u.Scheme)
	}

	return nil
}

// parseFileGitBarePath resolves a validated file:// clone URL to an absolute path.
func parseFileGitBarePath(cloneURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(cloneURL))
	if err != nil {
		return "", fmt.Errorf("invalid clone URL: %w", err)
	}

	if u.Scheme != schemeFile {
		return "", fmt.Errorf("not a file:// URL")
	}

	p := u.Path
	if p == "" {
		return "", fmt.Errorf("empty file path")
	}

	if strings.Contains(p, "\x00") || strings.ContainsAny(p, "\n\r") {
		return "", fmt.Errorf("file path contains invalid characters")
	}

	if strings.HasPrefix(p, "-") {
		return "", fmt.Errorf("file path must not start with '-'")
	}

	clean := filepath.Clean(filepath.FromSlash(p))
	if strings.HasPrefix(clean, "-") {
		return "", fmt.Errorf("file path must not start with '-'")
	}

	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}

	return abs, nil
}

func validateGitWorkdir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("empty workdir")
	}

	if strings.Contains(dir, "\x00") || strings.ContainsAny(dir, "\n\r") {
		return "", fmt.Errorf("workdir contains invalid characters")
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve workdir: %w", err)
	}

	clean := filepath.Clean(abs)
	if strings.HasPrefix(clean, "-") {
		return "", fmt.Errorf("workdir must not start with '-'")
	}

	return clean, nil
}

func validateGitConfigValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("empty git config value")
	}

	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("git config value must not start with '-'")
	}

	if strings.Contains(value, "\x00") {
		return fmt.Errorf("git config value contains null byte")
	}

	return nil
}

func validateGitCommitMessage(message string) error {
	if strings.Contains(message, "\x00") {
		return fmt.Errorf("commit message contains null byte")
	}

	return nil
}

func canonicalCloneURL(cloneURL string) (string, error) {
	if err := validateCloneURL(cloneURL); err != nil {
		return "", err
	}

	u, err := url.Parse(strings.TrimSpace(cloneURL))
	if err != nil {
		return "", fmt.Errorf("invalid clone URL: %w", err)
	}

	if u.Scheme != schemeFile {
		return strings.TrimSpace(cloneURL), nil
	}

	abs, err := parseFileGitBarePath(cloneURL)
	if err != nil {
		return "", err
	}

	return (&url.URL{Scheme: schemeFile, Path: abs}).String(), nil
}
