// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
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
	validation.SetAllowInsecureSinks(true)
	t.Cleanup(func() { validation.SetAllowInsecureSinks(false) })

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

// TestTLSConfigFromSpec_insecureDeniedByDefault pins the K-14 gate: no
// process-wide opt-in means insecureSkipVerify is refused at construction.
func TestTLSConfigFromSpec_insecureDeniedByDefault(t *testing.T) {
	validation.SetAllowInsecureSinks(false)
	t.Cleanup(func() { validation.SetAllowInsecureSinks(false) })

	if _, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil); err == nil {
		t.Fatal("expected insecureSkipVerify to be refused without --allow-insecure-sinks")
	}
}

func TestConfigFromSpec_requiresEndpoint(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "git"}, nil)
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
}

func TestValidateTLSSpec_requiresCASecretName(t *testing.T) {
	t.Parallel()

	err := ValidateTLSSpec(&kollectdevv1alpha1.TLSSpec{
		CASecretRef: &kollectdevv1alpha1.SecretReference{},
	})
	if err == nil {
		t.Fatal("expected error when caSecretRef.name is empty")
	}
}

func TestTLSConfigFromSpec_rejectsInvalidPEM(t *testing.T) {
	t.Parallel()

	_, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{
		CABundle: []byte("not-a-cert"),
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid CA bundle PEM")
	}
}
