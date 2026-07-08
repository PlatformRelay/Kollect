//go:build integration

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/forgejo"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"

	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportGitLabDirectPush(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	ctx := context.Background()
	container, err := forgejo.Run(ctx, "codeberg.org/forgejo/forgejo:11")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start forgejo: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	baseURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	user := container.AdminUsername()
	pass := container.AdminPassword()

	const repoName = "kollect-inventory"
	if err := createForgejoRepo(ctx, baseURL, user, pass, repoName); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	gitEndpoint := strings.TrimSuffix(baseURL, "/") + "/" + url.PathEscape(user) + "/" + repoName + ".git"
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: gitEndpoint,
	}

	backend, err := NewBackend(spec, nil, git.Auth{Username: user, Password: pass})
	if err != nil {
		t.Fatal(err)
	}

	const objectPath = "inventory/team-a/rollup.json"
	payload := []byte(`{"integration":true,"sink":"gitlab"}`)

	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	cloneURL, err := forgejoCloneURL(gitEndpoint, user, pass)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", cloneURL, clone).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone: %s: %v", out, err)
	}

	got, err := os.ReadFile(filepath.Join(clone, objectPath))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(payload) {
		t.Fatalf("file = %q, want %q", got, payload)
	}
}

func TestExportGitLabInventoryObjectPath(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	ctx := context.Background()
	container, err := forgejo.Run(ctx, "codeberg.org/forgejo/forgejo:11")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start forgejo: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	baseURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	user := container.AdminUsername()
	pass := container.AdminPassword()

	const repoName = "kollect-inventory-path"
	if err := createForgejoRepo(ctx, baseURL, user, pass, repoName); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	gitEndpoint := strings.TrimSuffix(baseURL, "/") + "/" + url.PathEscape(user) + "/" + repoName + ".git"
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: gitEndpoint,
	}

	backend, err := NewBackend(spec, nil, git.Auth{Username: user, Password: pass})
	if err != nil {
		t.Fatal(err)
	}

	const objectPath = "inventory/team-a/platform.json"
	payload := []byte(`{"schemaVersion":"v1alpha1","items":[]}`)

	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	cloneURL, err := forgejoCloneURL(gitEndpoint, user, pass)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", cloneURL, clone).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone: %s: %v", out, err)
	}

	got, err := os.ReadFile(filepath.Join(clone, objectPath))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(payload) {
		t.Fatalf("file = %q, want %q", got, payload)
	}
}

type forgejoRepoCreateRequest struct {
	Name          string `json:"name"`
	AutoInit      bool   `json:"auto_init"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// createForgejoRepo provisions the test repository, retrying while Forgejo
// finishes first-boot provisioning. The container's HTTP port accepts
// connections before the admin user is created, so the first requests can
// fail with connection-refused or HTTP 401 "user does not exist [uid: 0,
// name: forgejo-admin]" — both transient. Retry until the API is ready or the
// deadline elapses.
func createForgejoRepo(ctx context.Context, baseURL, user, pass, name string) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		err := attemptCreateForgejoRepo(ctx, baseURL, user, pass, name)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("create repo (giving up after retries): %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func attemptCreateForgejoRepo(ctx context.Context, baseURL, user, pass, name string) error {
	body, err := json.Marshal(forgejoRepoCreateRequest{
		Name:          name,
		AutoInit:      true,
		DefaultBranch: "main",
		Private:       false,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimSuffix(baseURL, "/")+"/api/v1/user/repos",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	// A prior attempt may have created the repo before its response reached
	// us; treat "already exists" as success so retries are idempotent.
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("create repo HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func TestExportGitLabMergeRequestMode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	ctx := context.Background()
	container, err := forgejo.Run(ctx, "codeberg.org/forgejo/forgejo:11")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start forgejo: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	baseURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	user := container.AdminUsername()
	pass := container.AdminPassword()

	const repoName = "kollect-inventory-mr"
	if err := createForgejoRepo(ctx, baseURL, user, pass, repoName); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	gitEndpoint := strings.TrimSuffix(baseURL, "/") + "/" + url.PathEscape(user) + "/" + repoName + ".git"
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: gitEndpoint,
		GitLab: &kollectdevv1alpha1.GitLabSpec{
			MergeRequest: &kollectdevv1alpha1.MergeRequestSpec{
				Mode:         "merge_request",
				TargetBranch: "main",
			},
		},
	}

	backend, err := NewBackend(spec, nil, git.Auth{Username: user, Password: pass, Token: pass})
	if err != nil {
		t.Fatal(err)
	}

	const objectPath = "inventory/team-a/platform.json"
	payload := []byte(`{"integration":true,"sink":"gitlab-mr"}`)
	featureBranch := BranchNameForExport("", "team-a", "platform")

	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export MR mode: %v", err)
	}

	mrs, err := listForgejoMergeRequests(ctx, baseURL, user, pass, user, repoName, featureBranch)
	if err != nil {
		t.Fatalf("list merge requests: %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("open MR count = %d, want 1", len(mrs))
	}
	if mrs[0].Head.Ref != featureBranch || mrs[0].Base.Ref != "main" {
		t.Fatalf("MR branches = head %q base %q, want source %q target main", mrs[0].Head.Ref, mrs[0].Base.Ref, featureBranch)
	}

	cloneURL, err := forgejoCloneURL(gitEndpoint, user, pass)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	cloneCmd := exec.Command( //nolint:gosec // G204: test fixture
		"git", "clone", "--branch", featureBranch, "--single-branch", cloneURL, clone,
	)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("clone feature branch: %s: %v", out, err)
	}

	got, err := os.ReadFile(filepath.Join(clone, objectPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("feature branch file = %q, want %q", got, payload)
	}
}

type forgejoMergeRequest struct {
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	State string `json:"state"`
}

func listForgejoMergeRequests(
	ctx context.Context,
	baseURL, user, pass, owner, repo, sourceBranch string,
) ([]forgejoMergeRequest, error) {
	project := url.PathEscape(owner) + "/" + url.PathEscape(repo)
	q := url.Values{}
	q.Set("state", "open")
	q.Set("head", sourceBranch)

	reqURL := strings.TrimSuffix(baseURL, "/") + "/api/v1/repos/" + project + "/pulls?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, pass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list pulls HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var records []forgejoMergeRequest
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, err
	}

	return records, nil
}

func forgejoCloneURL(gitEndpoint, user, pass string) (string, error) {
	u, err := url.Parse(gitEndpoint)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(user, pass)

	return u.String(), nil
}
