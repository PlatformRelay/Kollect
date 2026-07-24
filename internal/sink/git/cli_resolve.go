// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

// guardResolution pins the external transport to an answer set that passed the shared
// resolved-address policy. Every clone/fetch/push using this per-operation CLI
// environment therefore avoids a second, attacker-controlled DNS lookup.
func (c *cliEnv) guardResolution(ctx context.Context, endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse git endpoint for guarded resolution: %w", err)
	}
	switch u.Scheme {
	case schemeSSH:
		return c.guardSSHResolution(ctx, u)
	case schemeHTTP, schemeHTTPS:
		return c.guardHTTPResolution(ctx, u)
	case schemeFile:
		// Local filesystem remotes never dial; no DNS pin is required.
		return nil
	default:
		return fmt.Errorf("unsupported git endpoint scheme %q for guarded resolution", u.Scheme)
	}
}

func (c *cliEnv) guardHTTPResolution(ctx context.Context, u *url.URL) error {
	port := u.Port()
	if port == "" {
		if u.Scheme == schemeHTTP {
			port = "80"
		} else {
			port = "443"
		}
	}

	addresses, _, host, err := netguard.DefaultDialer.Resolve(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return err
	}
	for i, address := range addresses {
		idx := strconv.Itoa(i)
		c.extraEnv = append(c.extraEnv,
			"GIT_CONFIG_KEY_"+idx+"=http.curloptResolve",
			"GIT_CONFIG_VALUE_"+idx+"=+"+host+":"+port+":"+address.String(),
		)
	}
	c.extraEnv = append(c.extraEnv, "GIT_CONFIG_COUNT="+strconv.Itoa(len(addresses)))

	return nil
}

func (c *cliEnv) guardSSHResolution(ctx context.Context, u *url.URL) error {
	port := u.Port()
	if port == "" {
		port = "22"
	}
	addresses, _, host, err := netguard.DefaultDialer.Resolve(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return err
	}

	sshCommand := "ssh"
	for i, entry := range c.extraEnv {
		if value, ok := strings.CutPrefix(entry, "GIT_SSH_COMMAND="); ok {
			sshCommand = value
			c.extraEnv = append(c.extraEnv[:i], c.extraEnv[i+1:]...)
			break
		}
	}
	// All answers were authorized above. Pinning one numeric answer prevents
	// OpenSSH from performing a second DNS lookup; HostKeyAlias retains host-key
	// verification against the configured hostname rather than the numeric IP.
	sshCommand += " -o Hostname=" + shellQuote(addresses[0].String()) + " -o HostKeyAlias=" + shellQuote(host)
	c.extraEnv = append(c.extraEnv, "GIT_SSH_COMMAND="+sshCommand)

	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
