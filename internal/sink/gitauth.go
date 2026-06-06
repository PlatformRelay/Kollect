// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

const (
	secretKeyPassword   = "password"
	secretKeyToken      = "token"
	secretKeySSHKey     = "ssh-privatekey"
	secretKeyIdentity   = "identity"
	secretKeyIDRSA      = "id_rsa"
	secretKeyKnownHosts = "known_hosts"
)

// GitAuthFromSecretData maps standard secret keys to git sink credentials.
func GitAuthFromSecretData(data map[string][]byte, authType string) git.Auth {
	auth := git.Auth{}
	if data == nil {
		return auth
	}

	if v, ok := data["username"]; ok {
		auth.Username = string(v)
	}

	if v, ok := data[secretKeyPassword]; ok {
		auth.Password = string(v)
	}

	if v, ok := data[secretKeyToken]; ok {
		auth.Token = string(v)
	}

	auth.SSHPrivateKey = sshPrivateKeyFromSecret(data)
	auth.AuthType = gitAuthType(authType)

	return auth
}

// GitSSHKnownHostsFromSecretData returns OpenSSH known_hosts bytes from a secret.
func GitSSHKnownHostsFromSecretData(data map[string][]byte) []byte {
	if data == nil {
		return nil
	}

	if v, ok := data[secretKeyKnownHosts]; ok && len(v) > 0 {
		return v
	}

	return nil
}

func sshPrivateKeyFromSecret(data map[string][]byte) []byte {
	for _, key := range []string{secretKeySSHKey, secretKeyIdentity, secretKeyIDRSA} {
		if v, ok := data[key]; ok && len(v) > 0 {
			return v
		}
	}

	return nil
}

func gitAuthType(authType string) git.AuthType {
	switch authType {
	case kollectdevv1alpha1.GitAuthTypeSSH:
		return git.AuthTypeSSH
	default:
		return git.AuthTypeToken
	}
}

func gitAuthTypeFromSpec(spec kollectdevv1alpha1.KollectSinkSpec) string {
	if spec.Git != nil && spec.Git.Auth != nil {
		return spec.Git.Auth.Type
	}

	return ""
}
