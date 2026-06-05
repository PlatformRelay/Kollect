// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type TLSConfig struct {
	InsecureSkipVerify bool
	RootCAs            *x509.CertPool
}

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

func (c TLSConfig) ClientConfig() (*tls.Config, error) {
	if !c.Enabled() {
		return nil, nil
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec
		RootCAs:            c.RootCAs,
	}, nil
}

func (c TLSConfig) Enabled() bool {
	return c.InsecureSkipVerify || c.RootCAs != nil
}
