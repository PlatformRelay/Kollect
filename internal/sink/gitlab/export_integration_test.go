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

type forgejoRepoCreateRequest struct {
	Name          string `json:"name"`
	AutoInit      bool   `json:"auto_init"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

func createForgejoRepo(ctx context.Context, baseURL, user, pass, name string) error {
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
	if resp.StatusCode >= 300 {
		return fmt.Errorf("create repo HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func forgejoCloneURL(gitEndpoint, user, pass string) (string, error) {
	u, err := url.Parse(gitEndpoint)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(user, pass)

	return u.String(), nil
}
