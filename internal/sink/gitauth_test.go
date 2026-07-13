// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/git"
)

func TestGitSSHKnownHostsFromSecretData(t *testing.T) {
	t.Parallel()

	if got := GitSSHKnownHostsFromSecretData(nil); got != nil {
		t.Fatalf("nil data = %q", got)
	}

	data := map[string][]byte{"known_hosts": []byte("host key")}
	if got := GitSSHKnownHostsFromSecretData(data); string(got) != "host key" {
		t.Fatalf("got %q", got)
	}
}

func TestGitAuthFromSecretData(t *testing.T) {
	t.Parallel()

	if auth := GitAuthFromSecretData(nil, ""); auth.Username != "" || auth.Token != "" {
		t.Fatalf("nil data: %+v", auth)
	}

	auth := GitAuthFromSecretData(map[string][]byte{
		"username": []byte("bot"),
		"password": []byte("pw"),
		"token":    []byte("tok"),
	}, "token")
	if auth.Username != "bot" || auth.Password != "pw" || auth.Token != "tok" {
		t.Fatalf("auth = %+v", auth)
	}
}

func TestGitAuthFromSecretData_sshType(t *testing.T) {
	t.Parallel()

	// The ssh auth type selects git.AuthTypeSSH and reads the private key from secret keys.
	auth := GitAuthFromSecretData(map[string][]byte{
		"ssh-privatekey": []byte("PRIVATE-KEY"),
	}, kollectdevv1alpha1.GitAuthTypeSSH)
	if auth.AuthType != git.AuthTypeSSH {
		t.Fatalf("AuthType = %q, want %q", auth.AuthType, git.AuthTypeSSH)
	}
	if string(auth.SSHPrivateKey) != "PRIVATE-KEY" {
		t.Fatalf("SSHPrivateKey = %q", auth.SSHPrivateKey)
	}

	// Any other/empty auth type falls through to the token default.
	tokenAuth := GitAuthFromSecretData(map[string][]byte{}, "")
	if tokenAuth.AuthType != git.AuthTypeToken {
		t.Fatalf("default AuthType = %q, want %q", tokenAuth.AuthType, git.AuthTypeToken)
	}
}

func TestGitAuthTypeFromSpec(t *testing.T) {
	t.Parallel()

	// No git block → empty auth type (token fallthrough).
	if got := gitAuthTypeFromSpec(kollectdevv1alpha1.KollectSinkSpec{}); got != "" {
		t.Fatalf("no git block = %q, want empty", got)
	}

	// git block present but no auth → still empty.
	if got := gitAuthTypeFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Git: &kollectdevv1alpha1.GitSpec{},
	}); got != "" {
		t.Fatalf("git without auth = %q, want empty", got)
	}

	// git auth type is returned verbatim.
	if got := gitAuthTypeFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Git: &kollectdevv1alpha1.GitSpec{
			Auth: &kollectdevv1alpha1.GitAuthSpec{Type: kollectdevv1alpha1.GitAuthTypeSSH},
		},
	}); got != kollectdevv1alpha1.GitAuthTypeSSH {
		t.Fatalf("git ssh auth = %q, want %q", got, kollectdevv1alpha1.GitAuthTypeSSH)
	}
}
