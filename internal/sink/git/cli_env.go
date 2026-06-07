// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	envForceBasicAuth = "KOLLECT_GIT_FORCE_BASIC_AUTH"
	envAuthHeader     = "KOLLECT_GIT_AUTH_HEADER"
)

func forceBasicAuthFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envForceBasicAuth)))
	return v == "1" || v == "true" || v == "yes"
}

type cliEnv struct {
	extraEnv      []string
	configEnvArgs []string
	cleanupFns    []func()
}

func newCLIEnv(cfg Config, auth Auth, authType AuthType) (*cliEnv, error) {
	cli := &cliEnv{}

	if cfg.TLS.InsecureSkipVerify {
		cli.extraEnv = append(cli.extraEnv, "GIT_SSL_NO_VERIFY=true")
	}

	if cfg.ForceBasicAuth {
		if header := basicAuthHeader(auth); header != "" {
			cli.extraEnv = append(cli.extraEnv, envAuthHeader+"="+header)
			cli.configEnvArgs = append(cli.configEnvArgs, "--config-env", "http.extraHeader="+envAuthHeader)
		}
	}

	sshCfg := cfg.SSH
	if cfg.TLS.InsecureSkipVerify {
		sshCfg.InsecureSkipVerify = true
	}

	if authType == AuthTypeSSH || cfgNeedsCLISSH(cfg, authType) {
		sshCmd, cleanup, err := buildGitSSHCommand(auth, sshCfg)
		if err != nil {
			return nil, err
		}

		if sshCmd != "" {
			cli.extraEnv = append(cli.extraEnv, "GIT_SSH_COMMAND="+sshCmd)
		}

		if cleanup != nil {
			cli.cleanupFns = append(cli.cleanupFns, cleanup)
		}
	}

	return cli, nil
}

func (c *cliEnv) cleanup() {
	for _, fn := range c.cleanupFns {
		if fn != nil {
			fn()
		}
	}
}

func cfgNeedsCLISSH(cfg Config, authType AuthType) bool {
	if cfg.Engine != GitEngineCLI {
		return false
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return false
	}

	return u.Scheme == schemeSSH || authType == AuthTypeSSH
}

func applyCLIEnv(cmd *exec.Cmd, cli *cliEnv) {
	if cmd == nil {
		return
	}

	if cli != nil && len(cli.extraEnv) > 0 {
		cmd.Env = append(os.Environ(), cli.extraEnv...)
	}
}

func (c *cliEnv) prependGitArgs(args ...string) []string {
	if c == nil || len(c.configEnvArgs) == 0 {
		return args
	}

	out := make([]string, 0, len(c.configEnvArgs)+len(args))
	out = append(out, c.configEnvArgs...)
	out = append(out, args...)

	return out
}

func buildGitSSHCommand(auth Auth, sshCfg SSHConfig) (string, func(), error) {
	if len(auth.SSHPrivateKey) == 0 {
		if sshCfg.InsecureSkipVerify {
			return "ssh -o StrictHostKeyChecking=no", nil, nil
		}

		return "", nil, nil
	}

	keyPath, err := writeTempPrivateKey(auth.SSHPrivateKey)
	if err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = os.Remove(keyPath) }

	var opts []string
	opts = append(opts, "-i", keyPath, "-o", "IdentitiesOnly=yes")

	if sshCfg.InsecureSkipVerify {
		opts = append(opts, "-o", "StrictHostKeyChecking=no")
	} else if len(sshCfg.KnownHosts) > 0 {
		khPath, err := writeTempKnownHosts(sshCfg.KnownHosts)
		if err != nil {
			cleanup()

			return "", nil, err
		}

		prev := cleanup
		cleanup = func() {
			prev()
			_ = os.Remove(khPath)
		}
		opts = append(opts, "-o", "UserKnownHostsFile="+khPath)
	}

	return "ssh " + strings.Join(opts, " "), cleanup, nil
}

func writeTempPrivateKey(pem []byte) (string, error) {
	f, err := os.CreateTemp("", "kollect-git-identity-*")
	if err != nil {
		return "", fmt.Errorf("create ssh key temp file: %w", err)
	}

	path := f.Name()
	if _, err := f.Write(pem); err != nil {
		_ = f.Close()
		_ = os.Remove(path)

		return "", fmt.Errorf("write ssh key: %w", err)
	}

	if err := f.Chmod(0o600); err != nil { //nolint:gosec // G302: ssh key permissions
		_ = f.Close()
		_ = os.Remove(path)

		return "", fmt.Errorf("chmod ssh key: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(path)

		return "", fmt.Errorf("close ssh key temp file: %w", err)
	}

	return filepath.Clean(path), nil
}
