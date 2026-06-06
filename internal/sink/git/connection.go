// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const connectionTimeout = 15 * time.Second

// TestConnection verifies TLS to the git remote and optionally runs git ls-remote.
func TestConnection(ctx context.Context, cfg Config, auth Auth) error {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if u.Scheme == schemeHTTP {
		return lsRemote(ctx, cfg, auth)
	}

	if u.Scheme != "https" && u.Scheme != "" {
		return fmt.Errorf("unsupported URL scheme %q for TLS connection test", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint URL has no host")
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == schemeHTTP {
			port = "80"
		} else {
			port = "443"
		}
	}

	if err := tlsHandshake(ctx, host, port, cfg.TLS); err != nil {
		if cfg.TLS.RootCAs != nil {
			return fmt.Errorf(
				"TLS handshake failed: custom CA may be wrong or incomplete: %w",
				err,
			)
		}

		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	return lsRemote(ctx, cfg, auth)
}

func tlsHandshake(ctx context.Context, host, port string, tlsCfg TLSConfig) error {
	dialer := &net.Dialer{Timeout: connectionTimeout}
	addr := net.JoinHostPort(host, port)

	var conn *tls.Conn

	err := func() error {
		raw, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}

		clientTLS := tlsCfg.ClientTLSConfig()
		clientTLS.ServerName = host
		tlsConn := tls.Client(raw, clientTLS)

		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = raw.Close()

			return err
		}

		conn = tlsConn

		return nil
	}()
	if err != nil {
		return err
	}

	return conn.Close()
}

func lsRemote(ctx context.Context, cfg Config, auth Auth) error {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()

	endpoint := cfg.Endpoint
	if creds := auth.embedInURL(endpoint); creds != "" {
		endpoint = creds
	}

	//nolint:gosec // G204: endpoint validated at admission and ConfigFromSpec URL parse
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", endpoint)
	if cfg.TLS.InsecureSkipVerify {
		cmd.Env = append(cmd.Environ(), "GIT_SSL_NO_VERIFY=true")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git ls-remote failed: %s: %w", msg, err)
		}

		return fmt.Errorf("git ls-remote failed: %w", err)
	}

	return nil
}
