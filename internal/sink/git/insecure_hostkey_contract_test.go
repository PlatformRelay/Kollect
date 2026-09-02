// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// These tests pin the documented contract of spec.tls.insecureSkipVerify for the git sink
// (SEC-SSHHOSTKEY-01): the field is named for TLS but disables transport verification for
// whichever transport the sink's endpoint selects, and for an ssh:// endpoint that means SSH
// host-key verification. ADR-0104 and ADR-0407 were amended to say so; these tests keep the
// prose and the code from drifting apart again.

func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 host key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey: %v", err)
	}

	return sshPub
}

// TestEffectiveSSHConfig_TLSInsecureSkipVerifyDisablesHostKeyChecking pins the wiring itself:
// the TLS-named CRD field is copied onto SSHConfig.InsecureSkipVerify, which is what turns off
// host-key checking. effectiveSSHConfig is the single chokepoint used by the go-git export path
// (exportRemote) and by newCLIEnv, which the git-CLI export path and the connection-test path
// (lsRemoteUncached) both go through.
func TestEffectiveSSHConfig_TLSInsecureSkipVerifyDisablesHostKeyChecking(t *testing.T) {
	t.Parallel()

	knownHosts := []byte("git.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey\n")

	insecure := Config{
		TLS: TLSConfig{InsecureSkipVerify: true},
		SSH: SSHConfig{KnownHosts: knownHosts},
	}.effectiveSSHConfig()
	if !insecure.InsecureSkipVerify {
		t.Fatal("effectiveSSHConfig(): tls.insecureSkipVerify=true must set SSHConfig.InsecureSkipVerify")
	}
	if string(insecure.KnownHosts) != string(knownHosts) {
		t.Fatalf("effectiveSSHConfig(): KnownHosts = %q, want it carried through unchanged", insecure.KnownHosts)
	}

	secure := Config{
		TLS: TLSConfig{InsecureSkipVerify: false},
		SSH: SSHConfig{KnownHosts: knownHosts},
	}.effectiveSSHConfig()
	if secure.InsecureSkipVerify {
		t.Fatal("effectiveSSHConfig(): tls.insecureSkipVerify=false must leave SSHConfig.InsecureSkipVerify false")
	}
}

// TestSSHAuthMethod_InsecureSkipVerifyAcceptsAnyHostKey pins the go-git path behaviourally: with
// the flag set, the installed HostKeyCallback accepts a host key that is in no known_hosts file.
// The pre-existing TestSSHAuthMethod_insecure only asserted the callback is non-nil, which is true
// on both branches and so could not tell the secure callback from the insecure one.
func TestSSHAuthMethod_InsecureSkipVerifyAcceptsAnyHostKey(t *testing.T) {
	t.Parallel()

	auth, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("sshAuthMethod() error = %v", err)
	}
	if auth.HostKeyCallback == nil {
		t.Fatal("sshAuthMethod(): no HostKeyCallback installed")
	}

	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}
	if err := auth.HostKeyCallback("git.example:22", addr, testHostKey(t)); err != nil {
		t.Fatalf("insecure HostKeyCallback rejected an unknown host key (%v); "+
			"tls.insecureSkipVerify must install ssh.InsecureIgnoreHostKey", err)
	}
}

// TestSSHAuthMethod_KnownHostsCallbackRejectsWrongHostKey is the contrast case that makes the test
// above non-vacuous: without the flag, the callback built from known_hosts accepts the pinned key
// and rejects any other one.
func TestSSHAuthMethod_KnownHostsCallbackRejectsWrongHostKey(t *testing.T) {
	t.Parallel()

	pinned := testHostKey(t)
	known := []byte(knownhosts.Line([]string{"git.example:22"}, pinned) + "\n")

	auth, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{KnownHosts: known})
	if err != nil {
		t.Fatalf("sshAuthMethod() error = %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}
	if err := auth.HostKeyCallback("git.example:22", addr, pinned); err != nil {
		t.Fatalf("known_hosts HostKeyCallback rejected the pinned host key: %v", err)
	}
	if err := auth.HostKeyCallback("git.example:22", addr, testHostKey(t)); err == nil {
		t.Fatal("known_hosts HostKeyCallback accepted a host key that is not in known_hosts")
	}
}

// TestSSHAuthMethod_FailsClosedWithoutKnownHosts pins the guard rail the ADR amendments cite: the
// secure go-git path does not fall back to the system known_hosts or to trust-on-first-use, it
// refuses to build an auth method at all.
func TestSSHAuthMethod_FailsClosedWithoutKnownHosts(t *testing.T) {
	t.Parallel()

	auth, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{})
	if err == nil {
		t.Fatal("sshAuthMethod() error = nil, want fail-closed error without known_hosts")
	}
	if auth != nil {
		t.Fatal("sshAuthMethod() returned an auth method alongside the fail-closed error")
	}
	if !strings.Contains(err.Error(), "requires known_hosts") {
		t.Fatalf("sshAuthMethod() error = %v, want it to name the missing known_hosts", err)
	}
}

// TestNewCLIEnv_TLSInsecureSkipVerifyDisablesStrictHostKeyChecking pins the git-CLI engine, which
// is also the engine the connection test uses: lsRemoteUncached builds its environment with
// newCLIEnv, so a fix applied only to the go-git path would leave this path diverging.
func TestNewCLIEnv_TLSInsecureSkipVerifyDisablesStrictHostKeyChecking(t *testing.T) {
	t.Parallel()

	privateKey := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----")
	knownHosts := []byte("git.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey\n")

	sshCommand := func(t *testing.T, cfg Config) string {
		t.Helper()

		cli, err := newCLIEnv(cfg, Auth{SSHPrivateKey: privateKey}, AuthTypeSSH)
		if err != nil {
			t.Fatalf("newCLIEnv() error = %v", err)
		}
		t.Cleanup(cli.cleanup)

		for _, kv := range cli.extraEnv {
			if strings.HasPrefix(kv, "GIT_SSH_COMMAND=") {
				return strings.TrimPrefix(kv, "GIT_SSH_COMMAND=")
			}
		}

		t.Fatalf("newCLIEnv(): no GIT_SSH_COMMAND in extraEnv %v", cli.extraEnv)

		return ""
	}

	insecure := sshCommand(t, Config{
		Endpoint: "ssh://git@git.example/repo.git",
		Engine:   GitEngineCLI,
		TLS:      TLSConfig{InsecureSkipVerify: true},
		SSH:      SSHConfig{KnownHosts: knownHosts},
	})
	if !strings.Contains(insecure, "StrictHostKeyChecking=no") {
		t.Fatalf("GIT_SSH_COMMAND = %q, want StrictHostKeyChecking=no when tls.insecureSkipVerify is set", insecure)
	}

	secure := sshCommand(t, Config{
		Endpoint: "ssh://git@git.example/repo.git",
		Engine:   GitEngineCLI,
		TLS:      TLSConfig{InsecureSkipVerify: false},
		SSH:      SSHConfig{KnownHosts: knownHosts},
	})
	if strings.Contains(secure, "StrictHostKeyChecking=no") {
		t.Fatalf("GIT_SSH_COMMAND = %q, want host-key checking left on when tls.insecureSkipVerify is unset", secure)
	}
	if !strings.Contains(secure, "UserKnownHostsFile=") {
		t.Fatalf("GIT_SSH_COMMAND = %q, want the supplied known_hosts pinned via UserKnownHostsFile", secure)
	}
}
