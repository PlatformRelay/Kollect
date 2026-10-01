// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

// TLSConfig holds resolved TLS settings for git/HTTPS sinks (shared helper;
// HandshakeConfig is always non-nil because HTTPS is TLS by scheme).
type TLSConfig = sinktls.Config

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

// TLSConfigFromSpec builds TLSConfig from the sink spec and optional resolved CA
// PEM, enforcing the K-14 --allow-insecure-sinks gate.
func TLSConfigFromSpec(tlsSpec *kollectdevv1alpha1.TLSSpec, caPEM []byte) (TLSConfig, error) {
	return sinktls.FromSpec(tlsSpec, caPEM)
}
