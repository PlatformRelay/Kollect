// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportMemory(t *testing.T) {
	t.Parallel()

	hash, err := ExportMemory([]byte(`{"items":[]}`), "inventory/latest.json")
	if err != nil {
		t.Fatalf("ExportMemory() error = %v", err)
	}

	if hash.IsZero() {
		t.Fatal("expected non-zero commit hash")
	}
}

func TestExportFileRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	cmd := exec.Command("git", "init", "--bare", bare) //nolint:gosec // G204: test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	endpoint := "file://" + bare
	cfg := Config{Endpoint: endpoint}
	payload := []byte(`{"hello":"world"}`)

	if err := Export(t.Context(), cfg, Auth{}, payload, "inventory/test.json"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	cloneDir := filepath.Join(dir, "clone")
	cloneCmd := exec.Command( //nolint:gosec // G204: test fixture
		"git", "clone", "--branch", "main", "--single-branch", endpoint, cloneDir,
	)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %s: %v", out, err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "inventory", "test.json")) //nolint:gosec // G304: test clone dir
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}

	if string(data) != string(payload) {
		t.Fatalf("payload = %q, want %q", data, payload)
	}
}

func TestParseRemoteBranchFragment(t *testing.T) {
	t.Parallel()

	url, branch, err := parseRemote("https://example.com/r.git#branch=develop")
	if err != nil {
		t.Fatal(err)
	}

	if branch != "develop" {
		t.Fatalf("branch = %q", branch)
	}

	if url != "https://example.com/r.git" {
		t.Fatalf("url = %q", url)
	}
}

func TestResolveBranches(t *testing.T) {
	t.Parallel()

	clone, push := resolveBranches("main", nil)
	if clone != "main" || push != "main" {
		t.Fatalf("default = %q/%q", clone, push)
	}

	clone, push = resolveBranches("main", &BranchSpec{
		PushBranch:  "kollect/ns/inv",
		CloneBranch: "develop",
	})
	if clone != "develop" || push != "kollect/ns/inv" {
		t.Fatalf("feature branch = %q/%q", clone, push)
	}
}

func TestExportFileRemoteCommitPolicyOnPopulatedRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	initCmd := exec.Command("git", "init", "--bare", bare) //nolint:gosec // G204: test fixture
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	seedDir := filepath.Join(dir, "seed")
	cloneSeed := exec.Command("git", "clone", bare, seedDir) //nolint:gosec // G204: test fixture
	if out, err := cloneSeed.CombinedOutput(); err != nil {
		t.Fatalf("git clone seed: %s: %v", out, err)
	}

	for _, args := range [][]string{
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "checkout", "-b", "main"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "--allow-empty", "-m", "seed"},
		{"push", "-u", "origin", "main"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // G204: test fixture
		cmd.Dir = seedDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	endpoint := "file://" + bare
	cfg := Config{Endpoint: endpoint, PushPolicy: PushPolicyCommit}
	payload := []byte(`{"items":[{"uid":"u1"}]}`)

	if err := Export(t.Context(), cfg, Auth{}, payload, "inventory/test.json"); err != nil {
		t.Fatalf("export on populated remote: %v", err)
	}

	verifyDir := filepath.Join(dir, "verify")
	verifyClone := exec.Command( //nolint:gosec // G204: test fixture
		"git", "clone", "--branch", "main", "--single-branch", bare, verifyDir,
	)
	if out, err := verifyClone.CombinedOutput(); err != nil {
		t.Fatalf("git clone verify: %s: %v", out, err)
	}

	data, err := os.ReadFile(filepath.Join(verifyDir, "inventory", "test.json")) //nolint:gosec // G304: test clone dir
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}

	if string(data) != string(payload) {
		t.Fatalf("payload = %q, want %q", data, payload)
	}
}

func TestPushRefSpecCommitAvoidsForceOnPopulatedRemote(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		emptyRemote bool
		policy      PushPolicy
		wantForce   bool
	}{
		{name: "commit populated", emptyRemote: false, policy: PushPolicyCommit, wantForce: false},
		{name: "commit empty remote", emptyRemote: true, policy: PushPolicyCommit, wantForce: true},
		{name: "force populated", emptyRemote: false, policy: PushPolicyForcePush, wantForce: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := pushRefSpec("main", tc.emptyRemote, tc.policy)
			force := strings.HasPrefix(got, "+")
			if force != tc.wantForce {
				t.Fatalf("pushRefSpec() force = %v, want %v (ref=%q)", force, tc.wantForce, got)
			}
		})
	}
}

func TestExportFileRemoteForcePushResolvesNonFastForward(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	initCmd := exec.Command("git", "init", "--bare", bare) //nolint:gosec // G204: test fixture
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	endpoint := "file://" + bare
	cfg := Config{Endpoint: endpoint, PushPolicy: PushPolicyCommit}
	if err := Export(t.Context(), cfg, Auth{}, []byte(`{"base":true}`), "inventory/test.json"); err != nil {
		t.Fatalf("seed export: %v", err)
	}

	cloneA := filepath.Join(dir, "clone-a")
	cloneB := filepath.Join(dir, "clone-b")
	for _, dest := range []string{cloneA, cloneB} {
		cmd := exec.Command( //nolint:gosec // G204: test fixture
			"git", "clone", "--branch", "main", "--single-branch", bare, dest,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git clone %s: %s: %v", dest, out, err)
		}
	}

	writeAndCommit := func(dir string, content []byte, msg string) {
		t.Helper()

		target := filepath.Join(dir, "inventory", "test.json")
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}

		for _, args := range [][]string{
			{"add", "inventory/test.json"},
			{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", msg},
		} {
			cmd := exec.Command("git", args...) //nolint:gosec // G204: test fixture
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
			}
		}
	}

	writeAndCommit(cloneA, []byte(`{"winner":"a"}`), "advance remote")
	pushA := exec.Command("git", "push", "origin", "main") //nolint:gosec // G204: test fixture
	pushA.Dir = cloneA
	if out, err := pushA.CombinedOutput(); err != nil {
		t.Fatalf("git push clone-a: %s: %v", out, err)
	}

	writeAndCommit(cloneB, []byte(`{"stale":"b"}`), "stale divergent commit")
	pushB := exec.Command("git", "push", "origin", "main") //nolint:gosec // G204: test fixture
	pushB.Dir = cloneB
	if out, err := pushB.CombinedOutput(); err == nil {
		t.Fatalf("expected non-fast-forward push failure, got success: %s", out)
	}

	cfg.PushPolicy = PushPolicyForcePush
	if err := Export(t.Context(), cfg, Auth{}, []byte(`{"export":"force"}`), "inventory/test.json"); err != nil {
		t.Fatalf("ForcePush export: %v", err)
	}

	verifyDir := filepath.Join(dir, "verify")
	verifyClone := exec.Command( //nolint:gosec // G204: test fixture
		"git", "clone", "--branch", "main", "--single-branch", bare, verifyDir,
	)
	if out, err := verifyClone.CombinedOutput(); err != nil {
		t.Fatalf("git clone verify: %s: %v", out, err)
	}

	data, err := os.ReadFile(filepath.Join(verifyDir, "inventory", "test.json")) //nolint:gosec // G304: test clone dir
	if err != nil {
		t.Fatalf("read force-pushed file: %v", err)
	}

	if string(data) != `{"export":"force"}` {
		t.Fatalf("payload after force push = %q", data)
	}
}

func TestExportFileRemoteFeatureBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	cmd := exec.Command("git", "init", "--bare", bare) //nolint:gosec // G204: test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	endpoint := "file://" + bare
	cfg := Config{Endpoint: endpoint + "#branch=main"}
	payload := []byte(`{"hello":"feature"}`)

	if err := ExportWithBranch(t.Context(), cfg, Auth{}, payload, "inventory/feature.json", &BranchSpec{
		PushBranch:  "kollect/team-a/inventory",
		CloneBranch: "main",
	}, CommitContext{}); err != nil {
		t.Fatalf("ExportWithBranch() error = %v", err)
	}

	cloneDir := filepath.Join(dir, "clone")
	cloneCmd := exec.Command( //nolint:gosec // G204: test fixture
		"git", "clone", "--branch", "kollect/team-a/inventory", "--single-branch", endpoint, cloneDir,
	)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone feature branch: %s: %v", out, err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "inventory", "feature.json")) //nolint:gosec // G304: test clone dir
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}

	if string(data) != string(payload) {
		t.Fatalf("payload = %q, want %q", data, payload)
	}
}
