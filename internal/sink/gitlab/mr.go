// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// MergeRequestMode selects how inventory exports land in GitLab.
type MergeRequestMode string

const (
	// MergeRequestModeDirectPush pushes commits to the default branch (current scaffold behavior).
	MergeRequestModeDirectPush MergeRequestMode = "direct"
	// MergeRequestModeBranchMR pushes to a feature branch and opens/updates a merge request.
	MergeRequestModeBranchMR MergeRequestMode = "merge_request"
)

// MergeRequestConfig holds optional GitLab REST workflow settings (not yet in KollectSink CRD).
type MergeRequestConfig struct {
	Mode          MergeRequestMode
	TargetBranch  string
	BranchPrefix  string
	TitleTemplate string
	AutoMerge     bool
}

// ProjectRef identifies a GitLab project for REST API calls.
type ProjectRef struct {
	// Path is the URL-encoded namespace/project path (e.g. platform/kollect-inventory).
	Path string
	// ID is the numeric project ID when known; preferred by GitLab API v4.
	ID int
}

// ResolveProjectRef derives a project path from an HTTPS git remote endpoint.
func ResolveProjectRef(endpoint string) (ProjectRef, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return ProjectRef{}, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return ProjectRef{}, fmt.Errorf("gitlab endpoint must use https or http, got %q", u.Scheme)
	}

	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	path = strings.Trim(path, "/")
	if path == "" {
		return ProjectRef{}, fmt.Errorf("gitlab endpoint missing project path")
	}

	return ProjectRef{Path: path}, nil
}

// EnsureMergeRequest opens or updates a merge request for branch after a git push.
// Stub — full workflow deferred to Phase 2 (see docs/ROADMAP.md GitLab MR section).
func EnsureMergeRequest(_ context.Context, _ Config, _ MergeRequestConfig, _ string) error {
	return fmt.Errorf("gitlab: merge request workflow not implemented (direct push only)")
}
