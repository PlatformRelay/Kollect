// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// G2: auth method selection must match endpoint scheme and configured auth type.
func TestBuildAuthMethod_httpsToken(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethod("https://git.example/repo.git", Auth{
		Token: "secret",
	}, AuthTypeToken, SSHConfig{})
	if err != nil {
		t.Fatal(err)
	}

	basic, ok := method.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("method = %T", method)
	}
	if basic.Username != githubAccessTokenUser || basic.Password != "secret" {
		t.Fatalf("basic auth = %+v", basic)
	}
}

func TestBuildAuthMethod_httpsTokenWithExplicitUser(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethod("https://git.example/repo.git", Auth{
		Username: "bot",
		Token:    "secret",
	}, AuthTypeToken, SSHConfig{})
	if err != nil {
		t.Fatal(err)
	}

	basic, ok := method.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("method = %T", method)
	}
	if basic.Username != "bot" || basic.Password != "secret" {
		t.Fatalf("basic auth = %+v", basic)
	}
}

func TestBuildAuthMethod_sshRequiresKey(t *testing.T) {
	t.Parallel()

	_, err := buildAuthMethod("ssh://git@git.example/repo.git", Auth{}, AuthTypeSSH, SSHConfig{})
	if err == nil {
		t.Fatal("expected error for missing ssh key")
	}
}

func TestBuildAuthMethod_schemeTypeMismatch(t *testing.T) {
	t.Parallel()

	_, err := buildAuthMethod("https://git.example/repo.git", Auth{}, AuthTypeSSH, SSHConfig{})
	if err == nil {
		t.Fatal("expected error for ssh auth on https endpoint")
	}

	_, err = buildAuthMethod("ssh://git@git.example/repo.git", Auth{Token: "x"}, AuthTypeToken, SSHConfig{})
	if err == nil {
		t.Fatal("expected error for token auth on ssh endpoint")
	}
}

func TestBuildAuthMethod_fileSchemeNoAuth(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethod("file:///tmp/repo.git", Auth{}, AuthTypeToken, SSHConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if method != nil {
		t.Fatalf("file scheme auth = %#v, want nil", method)
	}
}

func TestBuildAuthMethod_sshWithKey(t *testing.T) {
	t.Parallel()

	keyPEM := testEd25519PrivateKeyPEM(t)
	method, err := buildAuthMethod("ssh://git@git.example/repo.git", Auth{
		SSHPrivateKey: keyPEM,
	}, AuthTypeSSH, SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if method == nil {
		t.Fatal("expected ssh auth method")
	}
}

func TestAuth_embedInURL_injectsCredentials(t *testing.T) {
	t.Parallel()

	auth := Auth{Username: "bot", Token: "tok"}
	got := auth.embedInURL("https://git.example/repo.git")
	if got == "" || got == "https://git.example/repo.git" {
		t.Fatalf("embedInURL = %q", got)
	}
	if auth.embedInURL("ssh://git@git.example/repo.git") != "" {
		t.Fatal("embedInURL should not modify ssh URLs")
	}
}

func TestBasicAuthHTTPS_emptyReturnsNil(t *testing.T) {
	t.Parallel()

	method, err := basicAuthHTTPS(Auth{})
	if err != nil {
		t.Fatal(err)
	}
	if method != nil {
		t.Fatalf("empty auth = %#v", method)
	}
}

func testEd25519PrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
