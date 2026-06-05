// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateTLSSpec(t *testing.T) {
	t.Parallel()

	err := ValidateTLSSpec(&kollectdevv1alpha1.TLSSpec{
		CABundle:    []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"),
		CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"},
	})
	if err == nil {
		t.Fatal("expected error when both caBundle and caSecretRef are set")
	}
}

func TestTLSConfigFromSpec_insecureSkip(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify true")
	}

	if !cfg.ClientTLSConfig().InsecureSkipVerify {
		t.Error("client config should inherit insecure skip verify")
	}
}

func TestConfigFromSpec_requiresEndpoint(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "git"}, nil)
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
}
