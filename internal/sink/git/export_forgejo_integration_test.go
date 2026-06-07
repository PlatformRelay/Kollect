//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

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
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportGoGit_nonFastForwardCommitPolicy_Forgejo(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, user, pass, gitEndpoint := startForgejoGit(t)
	const objectPath = "inventory/nonff.json"

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: gitEndpoint,
		Git:      &kollectdevv1alpha1.GitSpec{PushPolicy: kollectdevv1alpha1.GitPushPolicyCommit},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	auth := Auth{Username: user, Password: pass}
	if err := Export(ctx, cfg, auth, []byte(`{"seed":true}`), objectPath); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	cloneURL, err := forgejoCloneURL(gitEndpoint, user, pass)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	cloneA := filepath.Join(dir, "a")
	cloneB := filepath.Join(dir, "b")
	for _, dest := range []string{cloneA, cloneB} {
		cmd := exec.Command("git", "clone", "--branch", "main", "--single-branch", cloneURL, dest) //nolint:gosec // G204: test fixture
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("clone %s: %s: %v", dest, out, err)
		}
	}

	commitFile := func(repo string, body []byte, msg string) {
		t.Helper()
		target := filepath.Join(repo, objectPath)
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"add", objectPath},
			{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", msg},
		} {
			cmd := exec.Command("git", args...) //nolint:gosec // G204: test fixture
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %s: %v", args, out, err)
			}
		}
	}

	commitFile(cloneA, []byte(`{"winner":"a"}`), "advance")
	if out, err := exec.Command("git", "-C", cloneA, "push", "origin", "main").CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("push a: %s: %v", out, err)
	}

	commitFile(cloneB, []byte(`{"stale":"b"}`), "stale")
	if out, err := exec.Command("git", "-C", cloneB, "push", "origin", "main").CombinedOutput(); err == nil {
		t.Fatalf("expected non-fast-forward, got success: %s", out)
	}

	if err := Export(ctx, cfg, auth, []byte(`{"merged":true}`), objectPath); err != nil {
		t.Fatalf("export after divergence: %v", err)
	}

	verify := filepath.Join(dir, "verify")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", cloneURL, verify).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("verify clone: %s: %v", out, err)
	}

	got, err := os.ReadFile(filepath.Join(verify, objectPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"merged":true}` {
		t.Fatalf("remote payload = %q", got)
	}
}

func TestExportGit_authFailureTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, user, _, gitEndpoint := startForgejoGit(t)
	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: gitEndpoint,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = Export(ctx, cfg, Auth{Username: user, Password: "wrong-password"}, []byte(`{}`), "inventory/x.json")
	if err == nil {
		t.Fatal("expected auth failure")
	}

	if !kollecterrors.IsTerminal(ClassifyExportError(err)) {
		t.Fatalf("expected terminal auth error, got %v", err)
	}
}

func startForgejoGit(t *testing.T) (context.Context, string, string, string) {
	t.Helper()

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

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	baseURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	user := container.AdminUsername()
	pass := container.AdminPassword()
	const repoName = "kollect-git-nonff"
	if err := createForgejoRepo(ctx, baseURL, user, pass, repoName); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	gitEndpoint := strings.TrimSuffix(baseURL, "/") + "/" + url.PathEscape(user) + "/" + repoName + ".git"

	return ctx, user, pass, gitEndpoint
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
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(user, pass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("create repo HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
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
