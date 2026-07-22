// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"golang.org/x/crypto/ssh"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

func pinGoGitSSHResolution(
	ctx context.Context,
	cloneURL string,
	auth transport.AuthMethod,
) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil || u.Scheme != schemeSSH {
		return cloneURL, err
	}
	port := u.Port()
	if port == "" {
		port = "22"
	}
	addresses, _, host, err := netguard.DefaultDialer.Resolve(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return "", err
	}

	keyAuth, ok := auth.(*publicKeysAuth)
	if !ok {
		return "", fmt.Errorf("guard SSH resolution: unsupported auth method %T", auth)
	}
	if callback := keyAuth.HostKeyCallback; callback != nil {
		originalAddress := net.JoinHostPort(host, port)
		keyAuth.HostKeyCallback = func(_ string, remote net.Addr, key ssh.PublicKey) error {
			return callback(originalAddress, remote, key)
		}
	}

	u.Host = net.JoinHostPort(addresses[0].String(), port)

	return u.String(), nil
}
