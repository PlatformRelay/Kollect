// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultAPIVersion = "v4"

// HTTPClientTimeout bounds GitLab REST calls (matches git sink export timeout).
const HTTPClientTimeout = 2 * time.Minute

// MergeRequestAPI abstracts GitLab REST merge request operations (testable stub).
type MergeRequestAPI interface {
	EnsureOpenMergeRequest(
		ctx context.Context,
		project ProjectRef,
		sourceBranch, targetBranch, title string,
	) error
}

// RESTClient calls GitLab API v4 merge request endpoints.
type RESTClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewRESTClient builds a client for GitLab API v4 from a git remote endpoint and token.
func NewRESTClient(endpoint, token string, httpClient *http.Client) (*RESTClient, error) {
	base, err := APIBaseURL(endpoint)
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: HTTPClientTimeout}
	}

	return &RESTClient{
		BaseURL:    strings.TrimSuffix(base, "/"),
		Token:      strings.TrimSpace(token),
		HTTPClient: httpClient,
	}, nil
}

// APIBaseURL derives https://host/api/v4 from an HTTPS git remote endpoint.
func APIBaseURL(endpoint string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	if !isHTTPSEndpointScheme(u.Scheme) {
		return "", fmt.Errorf("gitlab API requires http(s) endpoint, got %q", u.Scheme)
	}

	return fmt.Sprintf("%s://%s/api/%s", u.Scheme, u.Host, defaultAPIVersion), nil
}

// projectPath returns the URL-encoded project identifier for API paths.
func projectPath(ref ProjectRef) (string, error) {
	if ref.ID > 0 {
		return fmt.Sprintf("%d", ref.ID), nil
	}
	if strings.TrimSpace(ref.Path) == "" {
		return "", fmt.Errorf("gitlab: project path or id required")
	}

	return url.PathEscape(ref.Path), nil
}

type mergeRequestRecord struct {
	IID          int    `json:"iid"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
}

// EnsureOpenMergeRequest finds an open MR for sourceBranch or creates one targeting targetBranch.
func (c *RESTClient) EnsureOpenMergeRequest(
	ctx context.Context,
	project ProjectRef,
	sourceBranch, targetBranch, title string,
) error {
	if c == nil {
		return fmt.Errorf("gitlab: REST client is nil")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("gitlab: merge request workflow requires api token in secretRef")
	}

	open, err := c.listOpenMergeRequests(ctx, project, sourceBranch)
	if err != nil {
		return err
	}
	for _, mr := range open {
		if mr.TargetBranch == targetBranch {
			return nil
		}
	}

	return c.createMergeRequest(ctx, project, sourceBranch, targetBranch, title)
}

func (c *RESTClient) listOpenMergeRequests(
	ctx context.Context,
	project ProjectRef,
	sourceBranch string,
) ([]mergeRequestRecord, error) {
	projectID, err := projectPath(project)
	if err != nil {
		return nil, err
	}

	records, apiErr := c.listGitLabMergeRequests(ctx, projectID, sourceBranch)
	if apiErr == nil {
		return records, nil
	}
	if !isGitLabAPIUnsupported(apiErr) {
		return nil, apiErr
	}

	return c.listGiteaPullRequests(ctx, project.Path, sourceBranch)
}

func (c *RESTClient) listGitLabMergeRequests(
	ctx context.Context,
	projectID, sourceBranch string,
) ([]mergeRequestRecord, error) {
	q := url.Values{}
	q.Set("state", "opened")
	q.Set("source_branch", sourceBranch)

	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests?%s", c.BaseURL, projectID, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.setGitLabAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab list merge requests: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab list merge requests: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}

	var records []mergeRequestRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("gitlab decode merge requests: %w", err)
	}

	return records, nil
}

func (c *RESTClient) listGiteaPullRequests(
	ctx context.Context,
	ownerRepo, sourceBranch string,
) ([]mergeRequestRecord, error) {
	q := url.Values{}
	q.Set("state", "open")
	if strings.TrimSpace(sourceBranch) != "" {
		q.Set("head", sourceBranch)
	}

	reqURL := fmt.Sprintf("%s/repos/%s/pulls?%s", c.giteaBaseURL(), ownerRepo, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.setGiteaAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitea list pull requests: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitea list pull requests: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}

	var records []mergeRequestRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("gitea decode pull requests: %w", err)
	}

	return records, nil
}

func (c *RESTClient) createMergeRequest(
	ctx context.Context,
	project ProjectRef,
	sourceBranch, targetBranch, title string,
) error {
	projectID, err := projectPath(project)
	if err != nil {
		return err
	}

	if err := c.createGitLabMergeRequest(ctx, projectID, sourceBranch, targetBranch, title); err == nil {
		return nil
	} else if !isGitLabAPIUnsupported(err) {
		return err
	}

	return c.createGiteaPullRequest(ctx, project.Path, sourceBranch, targetBranch, title)
}

func (c *RESTClient) createGitLabMergeRequest(
	ctx context.Context,
	projectID, sourceBranch, targetBranch, title string,
) error {
	payload := map[string]string{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"title":         title,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests", c.BaseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setGitLabAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab create merge request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab create merge request: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}

	return nil
}

func (c *RESTClient) createGiteaPullRequest(
	ctx context.Context,
	ownerRepo, sourceBranch, targetBranch, title string,
) error {
	payload := map[string]string{
		"head":          sourceBranch,
		"base":          targetBranch,
		"title":         title,
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/repos/%s/pulls", c.giteaBaseURL(), ownerRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setGiteaAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitea create pull request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gitea create pull request: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}

	return nil
}

func (c *RESTClient) giteaBaseURL() string {
	return strings.Replace(c.BaseURL, "/api/v4", "/api/v1", 1)
}

func isGitLabAPIUnsupported(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "HTTP 405") || strings.Contains(msg, "HTTP 404")
}

func (c *RESTClient) setGitLabAuth(req *http.Request) {
	if c.Token == "" {
		return
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
}

func (c *RESTClient) setGiteaAuth(req *http.Request) {
	if c.Token == "" {
		return
	}
	req.Header.Set("Authorization", "token "+c.Token)
}

func trimBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 240 {
		return s[:240] + "..."
	}

	return s
}
