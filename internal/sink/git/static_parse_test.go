// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// K-23 static-parse contract: a malformed endpoint that embeds credentials
// must fail with a static message that echoes neither the secret nor the host.
const leakyBadEndpoint = "http://user:s3cr3t@[::1"

func assertStatic(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("err = %v, want ErrInvalidEndpoint", err)
	}

	for _, leak := range []string{"s3cr3t", "user@", "[::1"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("error echoed %q: %q", leak, err.Error())
		}
	}
}

func TestConfigFromSpec_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: leakyBadEndpoint,
	}, nil)
	assertStatic(t, err)
}

func TestParseRemote_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	_, _, err := parseRemote(leakyBadEndpoint)
	assertStatic(t, err)
}

func TestParseEndpoint_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseEndpoint(leakyBadEndpoint)
	assertStatic(t, err)
}

func TestGuardResolution_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	cli := &cliEnv{}

	err := cli.guardResolution(context.Background(), leakyBadEndpoint)
	assertStatic(t, err)
}

// leakySpacedEndpoint carries a password with a space: url.Parse rejects it,
// and redact.Text cannot mask the run because the space ends its userinfo
// match, so only a static message keeps "cr3t" out of the error.
const leakySpacedEndpoint = "https://user:s3 cr3t@git.example.com/org/repo.git"

func assertStaticNoSecret(t *testing.T, err error) {
	t.Helper()

	assertStatic(t, err)

	for _, leak := range []string{"s3 cr3t", "cr3t", "user:"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("error echoed %q: %q", leak, err.Error())
		}
	}
}

func TestValidateCloneURL_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{leakyBadEndpoint, leakySpacedEndpoint} {
		assertStaticNoSecret(t, validateCloneURL(endpoint))
	}
}

func TestParseFileGitBarePath_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{leakyBadEndpoint, leakySpacedEndpoint} {
		_, err := parseFileGitBarePath(endpoint)
		assertStaticNoSecret(t, err)
	}
}

func TestCanonicalCloneURL_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{leakyBadEndpoint, leakySpacedEndpoint} {
		_, err := canonicalCloneURL(endpoint)
		assertStaticNoSecret(t, err)
	}
}

func TestTestConnection_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{leakyBadEndpoint, leakySpacedEndpoint} {
		err := TestConnection(context.Background(), Config{Endpoint: endpoint}, Auth{})
		assertStaticNoSecret(t, err)
	}
}
