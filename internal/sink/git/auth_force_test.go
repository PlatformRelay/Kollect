// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"net/http"
	"testing"
)

func TestBasicAuthHeader_token(t *testing.T) {
	t.Parallel()

	header := basicAuthHeader(Auth{Token: "secret-token"})
	if header == "" {
		t.Fatal("expected non-empty header")
	}

	method, err := buildAuthMethodWithForce(
		"https://github.com/org/repo.git",
		Auth{Token: "secret-token"},
		AuthTypeToken,
		SSHConfig{},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	fba, ok := method.(*forceBasicAuthMethod)
	if !ok {
		t.Fatalf("expected *forceBasicAuthMethod, got %T", method)
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	fba.SetAuth(req)

	if req.Header.Get("Authorization") == "" {
		t.Fatal("expected Authorization header")
	}

	if fba.Name() != "force-basic-auth" {
		t.Fatalf("Name() = %q", fba.Name())
	}
}

func TestBuildAuthMethodWithForce_disabledForSSH(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethodWithForce(
		"ssh://git@example.com/repo.git",
		Auth{SSHPrivateKey: testEd25519PrivateKeyPEM(t)},
		AuthTypeSSH,
		SSHConfig{InsecureSkipVerify: true},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := method.(*forceBasicAuthMethod); ok {
		t.Fatal("force basic auth must not apply to ssh://")
	}
}
