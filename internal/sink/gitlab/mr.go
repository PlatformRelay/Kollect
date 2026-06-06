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

// MergeRequestConfig holds optional GitLab REST workflow settings from spec.gitlab.mergeRequest.
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
	if !isHTTPSEndpointScheme(u.Scheme) {
		return ProjectRef{}, fmt.Errorf("gitlab endpoint must use https or http, got %q", u.Scheme)
	}

	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	path = strings.Trim(path, "/")
	if path == "" {
		return ProjectRef{}, fmt.Errorf("gitlab endpoint missing project path")
	}

	return ProjectRef{Path: path}, nil
}

// DefaultBranchPrefix is used when MergeRequestConfig.BranchPrefix is empty.
const DefaultBranchPrefix = "kollect"

// ValidateMergeRequestConfig checks MR settings before export.
func ValidateMergeRequestConfig(cfg MergeRequestConfig) error {
	switch cfg.Mode {
	case "", MergeRequestModeDirectPush:
		return nil
	case MergeRequestModeBranchMR:
		if strings.TrimSpace(cfg.TargetBranch) == "" {
			return fmt.Errorf("gitlab: merge_request mode requires targetBranch")
		}
		return nil
	default:
		return fmt.Errorf("gitlab: unknown merge request mode %q", cfg.Mode)
	}
}

// BranchNameForExport builds a feature branch path for inventory exports.
func BranchNameForExport(prefix, inventoryNamespace, inventoryName string) string {
	p := strings.Trim(strings.TrimSpace(prefix), "/")
	if p == "" {
		p = DefaultBranchPrefix
	}
	ns := strings.Trim(strings.TrimSpace(inventoryNamespace), "/")
	name := strings.Trim(strings.TrimSpace(inventoryName), "/")
	return fmt.Sprintf("%s/%s/%s", p, ns, name)
}

// MergeRequestTitle renders the MR title from a template or a default.
func MergeRequestTitle(template, inventoryNamespace, inventoryName string) string {
	tpl := strings.TrimSpace(template)
	if tpl == "" {
		return fmt.Sprintf("kollect inventory export: %s/%s", inventoryNamespace, inventoryName)
	}
	out := strings.ReplaceAll(tpl, "{namespace}", inventoryNamespace)
	return strings.ReplaceAll(out, "{name}", inventoryName)
}

// EnsureMergeRequest opens or updates a merge request for branch after a git push.
func EnsureMergeRequest(
	ctx context.Context,
	cfg Config,
	mrCfg MergeRequestConfig,
	sourceBranch, inventoryNamespace, inventoryName, apiToken, apiUser string,
) error {
	if err := ValidateMergeRequestConfig(mrCfg); err != nil {
		return err
	}
	if mrCfg.Mode != MergeRequestModeBranchMR {
		return nil
	}

	project, err := ResolveProjectRef(cfg.Endpoint)
	if err != nil {
		return err
	}

	client, err := NewRESTClient(cfg.Endpoint, apiToken, apiUser, nil)
	if err != nil {
		return err
	}

	title := MergeRequestTitle(mrCfg.TitleTemplate, inventoryNamespace, inventoryName)
	return client.EnsureOpenMergeRequest(ctx, project, sourceBranch, mrCfg.TargetBranch, title)
}
