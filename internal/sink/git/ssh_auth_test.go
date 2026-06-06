// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/ed25519"
	"crypto/rand"
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
