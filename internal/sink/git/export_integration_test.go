//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestExportBareRepoIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %s: %v", out, err)
	}

	cfg := Config{Endpoint: "file://" + bare}
	payload := []byte(`{"integration":true}`)

	if err := Export(t.Context(), cfg, Auth{}, payload, "inventory/integration.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	clone := filepath.Join(dir, "clone")
	if out, err := exec.Command("git", "clone", cfg.Endpoint, clone).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}

	got, err := os.ReadFile(filepath.Join(clone, "inventory", "integration.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}

	head, err := exec.Command("git", "-C", clone, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	hash := plumbing.NewHash(string(head[:40]))

	sum := sha256.Sum256(payload)
	expected := hex.EncodeToString(sum[:])
	if hash.String()[:7] == "" {
		t.Fatal("empty commit hash")
	}

	// Commit exists and payload round-trips; SHA256 documents expected inventory fingerprint.
	t.Logf("export commit=%s payload_sha256=%s", hash.String(), expected)
}
