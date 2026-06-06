// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
//
// Adapted from Argo CD Image Updater (Apache-2.0): ext/git/ssh.go, ext/git/client.go (newAuth)

package git

import (
	"fmt"
	"os"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHConfig controls SSH host key verification for go-git transport.
type SSHConfig struct {
	// InsecureSkipVerify disables host key checking (dev/test only).
	InsecureSkipVerify bool
	// KnownHosts holds OpenSSH known_hosts file contents. When empty and not
	// insecure, system default paths are not consulted — callers should supply
	// host keys via secret key "known_hosts".
	KnownHosts []byte
}

var defaultSSHKeyExchangeAlgorithms = []string{
	"curve25519-sha256",
	"curve25519-sha256@libssh.org",
	"ecdh-sha2-nistp256",
	"ecdh-sha2-nistp384",
	"ecdh-sha2-nistp521",
	"diffie-hellman-group-exchange-sha256",
	"diffie-hellman-group14-sha256",
	"diffie-hellman-group14-sha1",
}

type publicKeysAuth struct {
	gitssh.PublicKeys
}

func (a *publicKeysAuth) ClientConfig() (*ssh.ClientConfig, error) {
	config := ssh.Config{KeyExchanges: defaultSSHKeyExchangeAlgorithms}
	opts := &ssh.ClientConfig{
		Config: config,
		User:   a.User,
		Auth:   []ssh.AuthMethod{ssh.PublicKeys(a.Signer)},
	}

	return a.SetHostKeyCallback(opts)
}

func sshAuthMethod(user string, privateKey []byte, sshCfg SSHConfig) (*publicKeysAuth, error) {
	auth := &publicKeysAuth{}
	auth.User = user

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key: %w", err)
	}

	auth.Signer = signer

	if sshCfg.InsecureSkipVerify {
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // G106: dev/test only via tls.insecureSkipVerify

		return auth, nil
	}

	if len(sshCfg.KnownHosts) == 0 {
		return nil, fmt.Errorf("ssh git export requires known_hosts in secret or tls.insecureSkipVerify for dev")
	}

	path, err := writeTempKnownHosts(sshCfg.KnownHosts)
	if err != nil {
		return nil, err
	}

	defer func() { _ = os.Remove(path) }()

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts: %w", err)
	}

	auth.HostKeyCallback = callback

	return auth, nil
}

func writeTempKnownHosts(content []byte) (string, error) {
	f, err := os.CreateTemp("", "kollect-git-known-hosts-*")
	if err != nil {
		return "", fmt.Errorf("create known_hosts temp file: %w", err)
	}

	path := f.Name()

	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)

		return "", fmt.Errorf("write known_hosts: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(path)

		return "", fmt.Errorf("close known_hosts temp file: %w", err)
	}

	return path, nil
}
