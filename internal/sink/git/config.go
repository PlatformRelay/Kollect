// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// Config holds resolved git sink settings.
type Config struct {
	Endpoint string
	TLS      TLSConfig
	CABundle []byte
}

// ConfigFromSpec validates and resolves a KollectSink git spec.
func ConfigFromSpec(spec kollectdevv1alpha1.KollectSinkSpec, caPEM []byte) (Config, error) {
	if spec.Type != TypeName {
		return Config{}, fmt.Errorf("expected git sink, got %q", spec.Type)
	}

	if err := ValidateTLSSpec(spec.TLS); err != nil {
		return Config{}, err
	}

	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		return Config{}, fmt.Errorf("git sink requires spec.endpoint")
	}

	if _, err := url.Parse(endpoint); err != nil {
		return Config{}, fmt.Errorf("parse endpoint: %w", err)
	}

	tlsCfg, err := TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return Config{}, err
	}

	pem := caPEM
	if len(pem) == 0 && spec.TLS != nil {
		pem = spec.TLS.CABundle
	}

	return Config{
		Endpoint: endpoint,
		TLS:      tlsCfg,
		CABundle: pem,
	}, nil
}
