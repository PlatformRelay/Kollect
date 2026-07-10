//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

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
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/integrationtest"
	"github.com/konih/kollect/internal/sink/git"
)

// TestPipelineCLI_collectAndPushToGit is the L3 integration tier for the pipeline CLI (ADR-0801,
// P-008): it drives the real pipeline export path (ExportTargets) into a real git sink backend
// pushing to a Forgejo container. The collect->store path is covered by the L2 envtest test; this
// tier exercises the seam L2 and the unit tests do not — ExportTargets writing to a live git remote
// and the snapshot-store no-op guard (a byte-identical re-export must not create a second commit).
func TestPipelineCLI_collectAndPushToGit(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, user, pass, gitEndpoint := startForgejoGitPipeline(t)

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "inv",
		Namespace:       "default",
		Name:            "web",
		Kind:            "Deployment",
		UID:             "uid-1",
		Attributes:      map[string]any{"image": "nginx:1.25"},
	})

	target := kollectdevv1alpha1.KollectTarget{}
	target.Namespace = "default"
	target.Name = "inv"
	targets := []kollectdevv1alpha1.KollectTarget{target}

	sinkSpec := kollectdevv1alpha1.KollectSinkSpec{
		Type:         git.TypeName,
		Endpoint:     gitEndpoint,
		Cluster:      "test-cluster",
		PathTemplate: "inventory/{namespace}/{name}.json",
		Git:          &kollectdevv1alpha1.GitSpec{PushPolicy: kollectdevv1alpha1.GitPushPolicyCommit},
	}

	backend, err := git.NewBackend(sinkSpec, nil, git.Auth{Username: user, Password: pass}, nil)
	if err != nil {
		t.Fatalf("new git backend: %v", err)
	}

	// First export: collect payload is committed and pushed.
	exported, errs := ExportTargets(ctx, store, targets, backend, sinkSpec, "test-cluster", false)
	if len(errs) != 0 {
		t.Fatalf("first export errors: %v", errs)
	}
	if exported != 1 {
		t.Fatalf("first export exported = %d, want 1", exported)
	}

	cloneURL, err := forgejoCloneURLPipeline(gitEndpoint, user, pass)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	const objectPath = "inventory/default/inv.json"

	// The pushed inventory file must exist and be a valid export envelope carrying our attribute.
	firstClone := filepath.Join(dir, "first")
	gitClonePipeline(t, cloneURL, firstClone)

	got, err := os.ReadFile(filepath.Join(firstClone, objectPath)) //nolint:gosec // G304: path under t.TempDir()
	if err != nil {
		t.Fatalf("read pushed inventory: %v", err)
	}

	var env struct {
		ItemCount int `json:"itemCount"`
	}
	if err := json.Unmarshal(got, &env); err != nil {
		t.Fatalf("pushed inventory is not valid JSON: %v", err)
	}
	if env.ItemCount != 1 {
		t.Errorf("pushed itemCount = %d, want 1", env.ItemCount)
	}
	if !strings.Contains(string(got), "nginx:1.25") {
		t.Errorf("pushed inventory missing collected attribute; payload=%s", got)
	}

	// A second pipeline export must remain a valid, error-free operation (idempotent to re-run).
	// Note: it is not byte-identical to the first — ExportTargets embeds a per-run exportedAt
	// timestamp in the envelope — so it legitimately produces a new commit. The git no-op guard is
	// asserted separately below with content the test controls.
	exported2, errs2 := ExportTargets(ctx, store, targets, backend, sinkSpec, "test-cluster", false)
	if len(errs2) != 0 {
		t.Fatalf("second export errors: %v", errs2)
	}
	if exported2 != 1 {
		t.Fatalf("second export exported = %d, want 1", exported2)
	}

	// The git sink's no-op guard (export_file.go gitStatusClean) must skip the commit when the
	// pushed content is byte-identical. Drive it directly through the backend with a fixed payload
	// (the pipeline envelope's timestamp makes ExportTargets output non-identical across runs).
	const stablePath = "raw/stable.json"
	stable := []byte(`{"stable":"payload"}`)

	if err := backend.Export(ctx, stable, stablePath); err != nil {
		t.Fatalf("stable export 1: %v", err)
	}
	stableClone1 := filepath.Join(dir, "stable1")
	gitClonePipeline(t, cloneURL, stableClone1)
	commitsAfterStable := gitCommitCountPipeline(t, stableClone1)

	if err := backend.Export(ctx, stable, stablePath); err != nil {
		t.Fatalf("stable export 2 (byte-identical): %v", err)
	}
	stableClone2 := filepath.Join(dir, "stable2")
	gitClonePipeline(t, cloneURL, stableClone2)
	commitsAfterStable2 := gitCommitCountPipeline(t, stableClone2)

	if commitsAfterStable2 != commitsAfterStable {
		t.Fatalf("no-op guard failed: commit count went %d -> %d on byte-identical re-export",
			commitsAfterStable, commitsAfterStable2)
	}
}

func gitClonePipeline(t *testing.T, cloneURL, dest string) {
	t.Helper()

	cmd := exec.Command("git", "clone", "--branch", "main", "--single-branch", cloneURL, dest) //nolint:gosec // G204: test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone %s: %s: %v", dest, out, err)
	}
}

func gitCommitCountPipeline(t *testing.T, repo string) int {
	t.Helper()

	out, err := exec.Command("git", "-C", repo, "rev-list", "--count", "HEAD").CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("rev-list: %s: %v", out, err)
	}

	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		t.Fatalf("parse commit count %q: %v", out, err)
	}

	return n
}

// startForgejoGitPipeline mirrors the git sink's Forgejo helper (internal/sink/git/
// export_forgejo_integration_test.go). That helper is an unexported test symbol in another
// package, so it is duplicated here rather than imported (per P-008).
func startForgejoGitPipeline(t *testing.T) (context.Context, string, string, string) {
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
	const repoName = "kollect-pipeline-inventory"
	if err := createForgejoRepoPipeline(ctx, baseURL, user, pass, repoName); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	gitEndpoint := strings.TrimSuffix(baseURL, "/") + "/" + url.PathEscape(user) + "/" + repoName + ".git"

	return ctx, user, pass, gitEndpoint
}

type forgejoRepoCreateRequestPipeline struct {
	Name          string `json:"name"`
	AutoInit      bool   `json:"auto_init"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// createForgejoRepoPipeline provisions the test repo, retrying while Forgejo finishes first-boot
// provisioning (the HTTP port accepts connections before the admin user exists; early requests can
// fail with connection-refused or HTTP 401 "user does not exist" — both transient).
func createForgejoRepoPipeline(ctx context.Context, baseURL, user, pass, name string) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		err := attemptCreateForgejoRepoPipeline(ctx, baseURL, user, pass, name)
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

func attemptCreateForgejoRepoPipeline(ctx context.Context, baseURL, user, pass, name string) error {
	body, err := json.Marshal(forgejoRepoCreateRequestPipeline{
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

	// A prior attempt may have created the repo before its response reached us; treat
	// "already exists" as success so retries are idempotent.
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

		return fmt.Errorf("create repo HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return nil
}

func forgejoCloneURLPipeline(gitEndpoint, user, pass string) (string, error) {
	u, err := url.Parse(gitEndpoint)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(user, pass)

	return u.String(), nil
}
