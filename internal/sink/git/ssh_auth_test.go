// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHAuthMethod_insecure(t *testing.T) {
	t.Parallel()

	key := testEd25519PrivateKeyPEM(t)
	auth, err := sshAuthMethod("git", key, SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if auth == nil || auth.HostKeyCallback == nil {
		t.Fatal("expected host key callback")
	}
}

func TestSSHAuthMethod_requiresKnownHosts(t *testing.T) {
	t.Parallel()

	key := testEd25519PrivateKeyPEM(t)
	_, err := sshAuthMethod("git", key, SSHConfig{})
	if err == nil {
		t.Fatal("expected error without known_hosts")
	}
}

func TestSSHAuthMethod_knownHosts(t *testing.T) {
	t.Parallel()

	key := testEd25519PrivateKeyPEM(t)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	known := []byte(knownhosts.Line([]string{"git.example"}, sshPub) + "\n")
	_, err = sshAuthMethod("git", key, SSHConfig{KnownHosts: known})
	if err != nil {
		t.Fatal(err)
	}
}

// TestSSHAuthMethod_knownHostsWriteFailure drives the writeTempKnownHosts create-error branch (and its
// propagation out of sshAuthMethod): an unwritable TMPDIR makes os.CreateTemp fail, so the temp
// known_hosts file cannot be written and sshAuthMethod surfaces the wrapped error. Not parallel:
// t.Setenv mutates process env.
func TestSSHAuthMethod_knownHostsWriteFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	known := []byte(knownhosts.Line([]string{"git.example"}, sshPub) + "\n")

	_, err = sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{KnownHosts: known})
	if err == nil {
		t.Fatal("sshAuthMethod() error = nil, want wrapped known_hosts create failure")
	}
	if !strings.Contains(err.Error(), "create known_hosts temp file") {
		t.Fatalf("error = %v, want create known_hosts temp file wrapper", err)
	}
}

// TestSSHAuthMethod_malformedKnownHosts covers the knownhosts.New parse-error branch: content that is
// written successfully but is not a valid known_hosts line makes knownhosts.New fail.
func TestSSHAuthMethod_malformedKnownHosts(t *testing.T) {
	t.Parallel()

	_, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{KnownHosts: []byte("this is not a valid known_hosts line\n")})
	if err == nil {
		t.Fatal("sshAuthMethod() error = nil, want parse known_hosts error")
	}
	if !strings.Contains(err.Error(), "parse known_hosts") {
		t.Fatalf("error = %v, want parse known_hosts wrapper", err)
	}
}

// TestWriteTempKnownHosts_Success asserts the happy path writes the content verbatim to a temp file
// the caller can read back, and that the returned path is cleanable.
func TestWriteTempKnownHosts_Success(t *testing.T) {
	t.Parallel()

	content := []byte("git.example ssh-ed25519 AAAA...\n")
	path, err := writeTempKnownHosts(content)
	if err != nil {
		t.Fatalf("writeTempKnownHosts() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got, err := os.ReadFile(path) //nolint:gosec // G304: test reads the temp file it just wrote
	if err != nil {
		t.Fatalf("read temp known_hosts: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("known_hosts content = %q, want %q", got, content)
	}
}

func TestSSHAuthMethod_rejectsUnparseableKey(t *testing.T) {
	t.Parallel()

	_, err := sshAuthMethod("git", []byte("not a valid pem key"), SSHConfig{InsecureSkipVerify: true})
	if err == nil {
		t.Fatal("expected error for unparseable private key")
	}
	if !strings.Contains(err.Error(), "parse ssh private key") {
		t.Fatalf("error = %v, want parse ssh private key wrapper", err)
	}
}

func TestPublicKeysAuth_ClientConfig(t *testing.T) {
	t.Parallel()

	key := testEd25519PrivateKeyPEM(t)
	auth, err := sshAuthMethod("deploy", key, SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := auth.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.User != "deploy" {
		t.Fatalf("User = %q, want deploy", cfg.User)
	}
	if len(cfg.Auth) != 1 {
		t.Fatalf("Auth = %v, want one public-key auth method", cfg.Auth)
	}
	if cfg.HostKeyCallback == nil {
		t.Fatal("expected HostKeyCallback to be wired from the auth method")
	}
	if len(cfg.KeyExchanges) != len(defaultSSHKeyExchangeAlgorithms) {
		t.Fatalf("KeyExchanges = %v, want %v", cfg.KeyExchanges, defaultSSHKeyExchangeAlgorithms)
	}
}
