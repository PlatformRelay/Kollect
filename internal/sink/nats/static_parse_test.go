// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"errors"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// K-25: credentials in the NATS server URL are rejected at config time with a
// static message; a malformed URL likewise never echoes the raw input.
func natsSpec(url string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		Nats: &kollectdevv1alpha1.NatsSpec{URL: url, Subject: "events"},
	}
}

func assertNoEcho(t *testing.T, err error) {
	t.Helper()

	for _, leak := range []string{"s3cr3t", "user@", "[::1"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("error echoed %q: %q", leak, err.Error())
		}
	}
}

func TestConfigFromSpec_userinfoURLRejected(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(natsSpec("nats://user:s3cr3t@broker:4222"), nil)
	if !errors.Is(err, ErrURLCredentialsSet) {
		t.Fatalf("err = %v, want ErrURLCredentialsSet", err)
	}

	assertNoEcho(t, err)
}

func TestConfigFromSpec_userinfoInServerListRejected(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(natsSpec("nats://broker1:4222,nats://user:s3cr3t@broker2:4222"), nil)
	if !errors.Is(err, ErrURLCredentialsSet) {
		t.Fatalf("err = %v, want ErrURLCredentialsSet", err)
	}

	assertNoEcho(t, err)
}

func TestConfigFromSpec_malformedURLIsStatic(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(natsSpec("nats://user:s3cr3t@[::1"), nil)
	if !errors.Is(err, ErrInvalidServerURL) {
		t.Fatalf("err = %v, want ErrInvalidServerURL", err)
	}

	assertNoEcho(t, err)
}

func TestConfigFromSpec_cleanURLsAccepted(t *testing.T) {
	t.Parallel()

	for _, url := range []string{"nats://broker:4222", "tls://broker:4222", "nats://b1:4222,tls://b2:4222", "my-nats:4222"} {
		if _, err := ConfigFromSpec(natsSpec(url), nil); err != nil {
			t.Fatalf("ConfigFromSpec(%q) unexpected error: %v", url, err)
		}
	}
}
