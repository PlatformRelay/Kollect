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

func TestBuildAuthMethodWithForce_disabledWhenNotRequested(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethodWithForce(
		"https://github.com/org/repo.git",
		Auth{Token: "secret-token"},
		AuthTypeToken,
		SSHConfig{},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := method.(*forceBasicAuthMethod); ok {
		t.Fatal("force basic auth must not apply when useForceBasicAuth is false")
	}
}

func TestBuildAuthMethodWithForce_propagatesBuildError(t *testing.T) {
	t.Parallel()

	_, err := buildAuthMethodWithForce(
		"https://github.com/org/repo.git",
		Auth{},
		AuthTypeSSH,
		SSHConfig{},
		true,
	)
	if err == nil {
		t.Fatal("expected error from underlying buildAuthMethod to propagate")
	}
}

func TestBuildAuthMethodWithForce_emptyHeaderPassesThrough(t *testing.T) {
	t.Parallel()

	method, err := buildAuthMethodWithForce(
		"https://github.com/org/repo.git",
		Auth{},
		AuthTypeToken,
		SSHConfig{},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := method.(*forceBasicAuthMethod); ok {
		t.Fatal("force basic auth must not apply when there are no credentials")
	}
}

func TestForceBasicAuthMethod_String(t *testing.T) {
	t.Parallel()

	fba := &forceBasicAuthMethod{header: "Authorization: Basic x"}
	if fba.String() != fba.Name() {
		t.Fatalf("String() = %q, want %q", fba.String(), fba.Name())
	}
}

func TestForceBasicAuthMethod_SetAuth_noops(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()

		fba := &forceBasicAuthMethod{header: "Authorization: Basic x"}
		fba.SetAuth(nil) // must not panic
	})

	t.Run("empty header", func(t *testing.T) {
		t.Parallel()

		fba := &forceBasicAuthMethod{}
		req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
		fba.SetAuth(req)

		if req.Header.Get("Authorization") != "" {
			t.Fatal("expected no header set when forceBasicAuthMethod.header is empty")
		}
	})

	t.Run("malformed header without colon", func(t *testing.T) {
		t.Parallel()

		fba := &forceBasicAuthMethod{header: "no-colon-here"}
		req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
		fba.SetAuth(req)

		if len(req.Header) != 0 {
			t.Fatalf("expected no header set for malformed header, got %v", req.Header)
		}
	})
}
