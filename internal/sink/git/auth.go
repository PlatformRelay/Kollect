// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

const defaultGitUser = "git"

type Auth struct {
	Username      string
	Password      string
	Token         string
	SSHPrivateKey []byte
	AuthType      AuthType
}

func buildAuthMethod(cloneURL string, auth Auth, authType AuthType) (transport.AuthMethod, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case schemeHTTP, schemeHTTPS:
		if authType == AuthTypeSSH {
			return nil, fmt.Errorf("git auth type ssh requires an ssh:// endpoint")
		}

		return basicAuthHTTPS(auth)
	case schemeSSH:
		if authType == AuthTypeToken {
			return nil, fmt.Errorf("git auth type token requires an https:// endpoint")
		}

		return sshAuth(auth, u.User.Username())
	case schemeFile:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported git URL scheme %q", u.Scheme)
	}
}

func (a Auth) embedInURL(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != schemeHTTPS && u.Scheme != schemeHTTP {
		return ""
	}

	user := a.Username
	if user == "" {
		user = defaultGitUser
	}

	pass := strings.TrimSpace(a.Token)
	if pass == "" {
		pass = a.Password
	}

	if pass == "" {
		return ""
	}

	u.User = url.UserPassword(user, pass)

	return u.String()
}

func basicAuthHTTPS(auth Auth) (transport.AuthMethod, error) {
	if auth.Username == "" && auth.Token == "" && auth.Password == "" {
		return nil, nil
	}

	user := auth.Username
	if user == "" {
		user = defaultGitUser
	}

	pass := auth.Token
	if pass == "" {
		pass = auth.Password
	}

	return &githttp.BasicAuth{Username: user, Password: pass}, nil
}

func sshAuth(auth Auth, endpointUser string) (transport.AuthMethod, error) {
	if len(auth.SSHPrivateKey) == 0 {
		return nil, fmt.Errorf("ssh git export requires ssh-privatekey, identity, or id_rsa in secretRef")
	}

	user := strings.TrimSpace(auth.Username)
	if user == "" {
		user = strings.TrimSpace(endpointUser)
	}

	if user == "" {
		user = defaultGitUser
	}

	return gitssh.NewPublicKeys(user, auth.SSHPrivateKey, "")
}
