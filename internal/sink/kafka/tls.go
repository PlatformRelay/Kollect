// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
)

// TLSConfig holds resolved TLS settings for Kafka broker connections (K-13).
type TLSConfig struct {
	InsecureSkipVerify bool
	RootCAs            *x509.CertPool
}

// TLSConfigFromSpec builds TLSConfig from the sink spec and optional resolved CA
// PEM, mirroring the git/nats sinks. It refuses insecureSkipVerify unless the
// process-wide --allow-insecure-sinks opt-in is set (K-14).
func TLSConfigFromSpec(tlsSpec *kollectdevv1alpha1.TLSSpec, caPEM []byte) (TLSConfig, error) {
	cfg := TLSConfig{}
	if tlsSpec == nil {
		return cfg, nil
	}
	if tlsSpec.InsecureSkipVerify && !validation.AllowInsecureSinks() {
		return cfg, fmt.Errorf(
			"tls.insecureSkipVerify is not permitted: start the manager with --allow-insecure-sinks to opt in (K-14)",
		)
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

// ClientConfig returns a *tls.Config when any TLS material is configured, else
// nil so the broker connection stays plaintext (unchanged default).
func (c TLSConfig) ClientConfig() *tls.Config {
	if !c.Enabled() {
		return nil
	}

	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // gated by --allow-insecure-sinks (K-14)
		RootCAs:            c.RootCAs,
	}
}

// Enabled reports whether the config carries any TLS material.
func (c TLSConfig) Enabled() bool {
	return c.InsecureSkipVerify || c.RootCAs != nil
}
