// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
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

	if err := tlsHandshake(t.Context(), host, port, TLSConfig{InsecureSkipVerify: true}); err != nil {
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

	if err := tlsHandshake(t.Context(), host, port, TLSConfig{RootCAs: x509.NewCertPool()}); err == nil {
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
