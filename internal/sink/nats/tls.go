// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

// TLSConfig holds resolved TLS settings for nats connections. The connection
// stays plaintext unless TLS material is configured (ClientConfig returns nil).
type TLSConfig = sinktls.Config

// TLSConfigFromSpec resolves spec.tls via the shared sink TLS helper, which
// enforces the K-14 --allow-insecure-sinks gate.
func TLSConfigFromSpec(tlsSpec *kollectdevv1alpha1.TLSSpec, caPEM []byte) (TLSConfig, error) {
	return sinktls.FromSpec(tlsSpec, caPEM)
}
