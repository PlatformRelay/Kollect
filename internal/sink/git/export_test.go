// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
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
	cloneCmd := exec.Command("git", "clone", endpoint, cloneDir) //nolint:gosec // G204: test fixture
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
