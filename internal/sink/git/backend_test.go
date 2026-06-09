// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestBackendConfigAndType(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
	}
	b, err := NewBackend(spec, nil, Auth{Token: "tok"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Config().Endpoint != spec.Endpoint {
		t.Fatalf("Config() = %#v", b.Config())
	}
	if b.Capabilities().Stream {
		t.Fatal("git snapshot should not be stream emitter")
	}
}

func TestNewBackend_SetsSSHKnownHosts(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "ssh://git@example.com/inventory.git",
	}
	knownHosts := []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey")
	b, err := NewBackend(spec, nil, Auth{}, knownHosts)
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	if string(b.cfg.SSH.KnownHosts) != string(knownHosts) {
		t.Fatalf("KnownHosts = %q, want %q", b.cfg.SSH.KnownHosts, knownHosts)
	}
}

func TestBackend_ExportFiles_PruneRemovesStaleEntries(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "repo.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	seed := filepath.Join(dir, "seed")
	if out, err := exec.Command("git", "clone", bare, seed).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone seed: %s: %v", out, err)
	}
	for _, rel := range []string{"inventory/stale.json", "inventory/keep.json"} {
		full := filepath.Join(seed, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(`{"seed":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "seed"},
		{"branch", "-M", "main"},
		{"push", "-u", "origin", "main"},
	} {
		cmd := exec.Command("git", args...) //nolint:gosec // G204: test fixture
		cmd.Dir = seed
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "file://" + bare,
	}
	backend, err := NewBackend(spec, nil, Auth{}, nil)
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	if exportErr := backend.ExportFiles(t.Context(), []FileEntry{{
		Path: "inventory/keep.json",
		Data: []byte(`{"fresh":true}`),
	}}, true); exportErr != nil {
		t.Fatalf("ExportFiles() error = %v", exportErr)
	}

	verify := filepath.Join(dir, "verify")
	if out, cloneErr := exec.Command("git", "clone", "--branch", "main", "--single-branch", bare, verify).CombinedOutput(); cloneErr != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone verify: %s: %v", out, cloneErr)
	}

	if _, staleErr := os.Stat(filepath.Join(verify, "inventory", "stale.json")); !os.IsNotExist(staleErr) {
		t.Fatalf("stale file should be pruned, stat err=%v", staleErr)
	}
	data, err := os.ReadFile(filepath.Join(verify, "inventory", "keep.json")) //nolint:gosec // G304: test fixture
	if err != nil {
		t.Fatalf("read keep.json: %v", err)
	}
	if string(data) != `{"fresh":true}` {
		t.Fatalf("keep.json payload = %q", data)
	}
}
