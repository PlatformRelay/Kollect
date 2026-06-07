// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
//
// Adapted from Argo CD Image Updater (Apache-2.0): ext/git/client.go, ext/git/creds.go

package git

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	defaultGitUser        = "git"
	githubAccessTokenUser = "x-access-token"
)

type Auth struct {
	Username      string
	Password      string
	Token         string
	SSHPrivateKey []byte
	AuthType      AuthType
}

func buildAuthMethod(cloneURL string, auth Auth, authType AuthType, sshCfg SSHConfig) (transport.AuthMethod, error) {
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

		return sshAuth(auth, u.User.Username(), sshCfg)
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
	pass := auth.Token
	if pass == "" {
		pass = auth.Password
	}

	if user == "" {
		if pass != "" && auth.Token != "" {
			user = githubAccessTokenUser
		} else {
			user = defaultGitUser
		}
	}

	return &githttp.BasicAuth{Username: user, Password: pass}, nil
}

func sshAuth(auth Auth, endpointUser string, sshCfg SSHConfig) (transport.AuthMethod, error) {
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

	return sshAuthMethod(user, auth.SSHPrivateKey, sshCfg)
}

func buildAuthMethodWithForce(
	cloneURL string,
	auth Auth,
	authType AuthType,
	sshCfg SSHConfig,
	useForceBasicAuth bool,
) (transport.AuthMethod, error) {
	method, err := buildAuthMethod(cloneURL, auth, authType, sshCfg)
	if err != nil || !useForceBasicAuth {
		return method, err
	}

	u, err := url.Parse(cloneURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return method, nil
	}

	header := basicAuthHeader(auth)
	if header == "" {
		return method, nil
	}

	return &forceBasicAuthMethod{header: header}, nil
}

func basicAuthHeader(auth Auth) string {
	user := auth.Username
	pass := auth.Token
	if pass == "" {
		pass = auth.Password
	}

	if pass == "" && user == "" {
		return ""
	}

	if user == "" {
		if auth.Token != "" {
			user = githubAccessTokenUser
		} else {
			user = defaultGitUser
		}
	}

	creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))

	return "Authorization: Basic " + creds
}

type forceBasicAuthMethod struct {
	header string
}

func (a *forceBasicAuthMethod) Name() string {
	return "force-basic-auth"
}

func (a *forceBasicAuthMethod) String() string {
	return a.Name()
}

func (a *forceBasicAuthMethod) SetAuth(req *http.Request) {
	if req == nil || a.header == "" {
		return
	}

	parts := strings.SplitN(a.header, ":", 2)
	if len(parts) != 2 {
		return
	}

	req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
}
