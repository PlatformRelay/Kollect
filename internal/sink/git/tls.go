// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TLSConfig holds resolved TLS settings for git/HTTPS sinks.
type TLSConfig struct {
	InsecureSkipVerify bool
	RootCAs            *x509.CertPool
}

// ValidateTLSSpec rejects ambiguous TLS configuration.
func ValidateTLSSpec(tlsSpec *kollectdevv1alpha1.TLSSpec) error {
	if tlsSpec == nil {
		return nil
	}

	if len(tlsSpec.CABundle) > 0 && tlsSpec.CASecretRef != nil {
		return fmt.Errorf("tls: set either caBundle or caSecretRef, not both")
	}

	if tlsSpec.CASecretRef != nil && tlsSpec.CASecretRef.Name == "" {
		return fmt.Errorf("tls.caSecretRef.name is required")
	}

	return nil
}

// TLSConfigFromSpec builds TLSConfig from the sink spec and optional resolved CA PEM.
func TLSConfigFromSpec(tlsSpec *kollectdevv1alpha1.TLSSpec, caPEM []byte) (TLSConfig, error) {
	cfg := TLSConfig{}

	if tlsSpec == nil {
		return cfg, nil
	}

	cfg.InsecureSkipVerify = tlsSpec.InsecureSkipVerify

	pem := caPEM
	if len(pem) == 0 {
		pem = tlsSpec.CABundle
	}

	if len(pem) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return cfg, fmt.Errorf("tls: failed to parse CA bundle PEM")
		}

		cfg.RootCAs = pool
	}

	return cfg, nil
}

// ClientTLSConfig returns a crypto/tls.Config for outbound connections.
func (c TLSConfig) ClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // user-controlled for private CAs
		RootCAs:            c.RootCAs,
		MinVersion:         tls.VersionTLS12,
	}
}
