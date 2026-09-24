// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
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

func TestAPIBaseURL_malformedEndpointIsStatic(t *testing.T) {
	t.Parallel()

	_, err := APIBaseURL(leakyBadEndpoint)
	assertStatic(t, err)
}
