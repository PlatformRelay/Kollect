// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteTempPrivateKey_PermsAndContent locks the SSH identity temp-file contract: the key material
// is written verbatim and the file is chmod 0600 (owner-only). A regression to a laxer mode (e.g.
// 0644) or a truncated/garbled write would fail this assertion, so it is not a tautology.
func TestWriteTempPrivateKey_PermsAndContent(t *testing.T) {
	t.Parallel()

	pem := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ndeadbeefcafebabe\n-----END OPENSSH PRIVATE KEY-----\n")

	path, err := writeTempPrivateKey(pem)
	if err != nil {
		t.Fatalf("writeTempPrivateKey() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat temp key: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("temp key perms = %o, want 0600 (owner read/write only)", perm)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: test reads the temp file it just wrote
	if err != nil {
		t.Fatalf("read temp key: %v", err)
	}
	if !bytes.Equal(got, pem) {
		t.Fatalf("temp key content = %q, want %q", got, pem)
	}
}

// TestWriteTempPrivateKey_CreateFailureIsWrapped covers the CreateTemp error branch: an unwritable
// TMPDIR makes os.CreateTemp fail and the wrapper must surface a "create ssh key temp file" error
// without leaking a path. Not parallel: t.Setenv mutates process env.
func TestWriteTempPrivateKey_CreateFailureIsWrapped(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))

	path, err := writeTempPrivateKey([]byte("key"))
	if err == nil {
		_ = os.Remove(path)
		t.Fatal("writeTempPrivateKey() error = nil, want create-temp failure")
	}
	if path != "" {
		t.Fatalf("writeTempPrivateKey() path = %q, want empty on failure", path)
	}
	if !strings.Contains(err.Error(), "create ssh key temp file") {
		t.Fatalf("error = %v, want create ssh key temp file wrapper", err)
	}
}
