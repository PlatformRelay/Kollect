// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"
	"testing"
)

// stubAuthMethod is a transport.AuthMethod that is not *publicKeysAuth, used to drive the
// unsupported-auth-method guard branch of pinGoGitSSHResolution.
type stubAuthMethod struct{}

func (stubAuthMethod) Name() string   { return "stub" }
func (stubAuthMethod) String() string { return "stub" }

// TestPinGoGitSSHResolution_RewritesToResolvedLiteral drives the happy path with a public IP literal
// so netguard resolves it without any DNS/network, and asserts the clone URL is rewritten to the
// resolved host:port. The input omits the port, so the ":22" default in the output is an observable
// rewrite (not a tautology). A *publicKeysAuth with a host-key callback also exercises the callback
// rebinding branch.
func TestPinGoGitSSHResolution_RewritesToResolvedLiteral(t *testing.T) {
	t.Parallel()

	auth, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("sshAuthMethod() error = %v", err)
	}
	if auth.HostKeyCallback == nil {
		t.Fatal("precondition: expected a host-key callback to exercise the rebinding branch")
	}

	got, err := pinGoGitSSHResolution(t.Context(), "ssh://git@8.8.8.8/repo.git", auth)
	if err != nil {
		t.Fatalf("pinGoGitSSHResolution() error = %v", err)
	}
	if !strings.Contains(got, "8.8.8.8:22") {
		t.Fatalf("pinGoGitSSHResolution() = %q, want host rewritten to 8.8.8.8:22", got)
	}
}

// TestPinGoGitSSHResolution_NonSSHSchemeUnchanged covers the early return for non-ssh URLs: the input
// is returned verbatim with no error and no resolution attempted.
func TestPinGoGitSSHResolution_NonSSHSchemeUnchanged(t *testing.T) {
	t.Parallel()

	const in = "https://example.com/repo.git"
	got, err := pinGoGitSSHResolution(t.Context(), in, nil)
	if err != nil {
		t.Fatalf("pinGoGitSSHResolution() error = %v, want nil for non-ssh scheme", err)
	}
	if got != in {
		t.Fatalf("pinGoGitSSHResolution() = %q, want unchanged %q", got, in)
	}
}

// TestPinGoGitSSHResolution_UnparseableURL covers the url.Parse error branch.
func TestPinGoGitSSHResolution_UnparseableURL(t *testing.T) {
	t.Parallel()

	if _, err := pinGoGitSSHResolution(t.Context(), "ssh://\x7f bad", nil); err == nil {
		t.Fatal("pinGoGitSSHResolution() error = nil, want parse error for malformed URL")
	}
}

// TestPinGoGitSSHResolution_ResolveFailureReturnsError covers the netguard resolve-failure branch: a
// forbidden hostname (localhost) is rejected by the guard, so the guarded URL is empty and the error
// surfaces rather than being swallowed.
func TestPinGoGitSSHResolution_ResolveFailureReturnsError(t *testing.T) {
	t.Parallel()

	auth, err := sshAuthMethod("git", testEd25519PrivateKeyPEM(t), SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("sshAuthMethod() error = %v", err)
	}

	got, err := pinGoGitSSHResolution(t.Context(), "ssh://git@localhost/repo.git", auth)
	if err == nil {
		t.Fatal("pinGoGitSSHResolution() error = nil, want forbidden-host resolve error")
	}
	if got != "" {
		t.Fatalf("pinGoGitSSHResolution() = %q, want empty on resolve failure", got)
	}
}

// TestPinGoGitSSHResolution_UnsupportedAuthMethod covers the type-assertion guard: a non-publicKeysAuth
// method yields a wrapped error naming the unsupported type.
func TestPinGoGitSSHResolution_UnsupportedAuthMethod(t *testing.T) {
	t.Parallel()

	_, err := pinGoGitSSHResolution(t.Context(), "ssh://git@8.8.8.8/repo.git", stubAuthMethod{})
	if err == nil {
		t.Fatal("pinGoGitSSHResolution() error = nil, want unsupported auth method error")
	}
	if !strings.Contains(err.Error(), "unsupported auth method") {
		t.Fatalf("error = %v, want unsupported auth method wrapper", err)
	}
}
