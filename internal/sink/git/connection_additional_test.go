// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestTestConnection_FileEndpointSuccess(t *testing.T) {
	remoteDir := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remoteDir}

	if err := TestConnection(t.Context(), cfg, Auth{}); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestTestConnection_UnsupportedScheme(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), Config{Endpoint: "ftp://example.com/repo.git"}, Auth{})
	if err == nil || !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Fatalf("TestConnection() error = %v, want unsupported scheme", err)
	}
}

func TestTestConnection_NoHost(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), Config{Endpoint: "https:///repo.git"}, Auth{})
	if err == nil || !strings.Contains(err.Error(), "no host") {
		t.Fatalf("TestConnection() error = %v, want missing host error", err)
	}
}

func TestTestConnection_TLSHandshakeErrorMentionsCustomCA(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), Config{
		Endpoint: "https://127.0.0.1:1/repo.git",
		TLS: TLSConfig{
			RootCAs: x509.NewCertPool(),
		},
	}, Auth{})
	if err == nil || !strings.Contains(err.Error(), "custom CA may be wrong or incomplete") {
		t.Fatalf("TestConnection() error = %v, want custom CA hint", err)
	}
}

// TestTestConnection_HTTPSchemeUsesLsRemote covers the plain-http branch of
// TestConnection, which must skip the TLS handshake entirely and probe via ls-remote.
// Pointing at a closed port yields a classified error, proving the branch was taken
// (a TLS-handshake path would report a handshake error instead).
func TestTestConnection_HTTPSchemeUsesLsRemote(t *testing.T) {
	t.Parallel()

	lsRemoteRefCache = newRefCache(defaultRefCacheTTL)
	// Reserved TEST-NET-1 address / closed port -> ls-remote fails fast.
	cfg := Config{Endpoint: "http://192.0.2.1:1/repo.git"}

	err := TestConnection(t.Context(), cfg, Auth{})
	if err == nil {
		t.Fatal("expected error probing an unreachable http remote")
	}
	if strings.Contains(err.Error(), "TLS handshake") {
		t.Fatalf("http scheme must not attempt a TLS handshake, got %v", err)
	}
}

// TestLSRemoteUncached_FormatsGitFailure exercises the error-wrapping path in
// lsRemoteUncached: ls-remote against a non-existent file:// repo fails and the
// combined git output must be surfaced in the wrapped error.
func TestLSRemoteUncached_FormatsGitFailure(t *testing.T) {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		t.Skip("git not in PATH")
	}

	missing := t.TempDir() + "/does-not-exist.git"
	cfg := Config{Endpoint: "file://" + missing}.withDefaults()

	err := lsRemoteUncached(t.Context(), cfg, Auth{})
	if err == nil {
		t.Fatal("expected ls-remote failure against a missing repository")
	}
	if !strings.Contains(err.Error(), "git ls-remote failed") {
		t.Fatalf("error = %v, want git ls-remote failed wrapper", err)
	}
}

func TestTLSHandshake_succeedsWithInsecureSkipVerify(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if err := tlsHandshakeWithDialer(t.Context(), host, port, TLSConfig{InsecureSkipVerify: true}, &net.Dialer{}); err != nil {
		t.Fatalf("tlsHandshake() error = %v, want success", err)
	}
}

func TestTLSHandshake_failsOnUntrustedCert(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if err := tlsHandshakeWithDialer(t.Context(), host, port, TLSConfig{RootCAs: x509.NewCertPool()}, &net.Dialer{}); err == nil {
		t.Fatal("expected handshake error for untrusted certificate")
	}
}

func TestLSRemote_CachesResult(t *testing.T) {
	t.Parallel()

	lsRemoteRefCache = newRefCache(defaultRefCacheTTL)
	cfg := Config{Endpoint: "https://127.0.0.1:1/repo.git"}

	first := lsRemote(t.Context(), cfg, Auth{})
	second := lsRemote(t.Context(), cfg, Auth{})
	if first == nil || second == nil {
		t.Fatalf("lsRemote() first=%v second=%v, want cached error", first, second)
	}
	if first.Error() != second.Error() {
		t.Fatalf("cached errors differ: first=%q second=%q", first.Error(), second.Error())
	}
}
