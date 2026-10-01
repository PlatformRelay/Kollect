// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/netguard"
)

// TestConnection requests broker metadata to verify reachability.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
	caPEM []byte,
) error {
	cfg, err := ConfigFromSpec(spec, secretData)
	if err != nil {
		return err
	}

	tlsCfg, err := TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return err
	}

	transport, err := dialTransport(cfg, tlsCfg)
	if err != nil {
		return err
	}

	conn, err := probeDialer(tlsCfg, transport).DialContext(ctx, "tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("kafka dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Brokers(); err != nil {
		return fmt.Errorf("kafka metadata: %w", err)
	}

	return nil
}

// probeDialer builds the metadata-probe dialer with the same netguard dial path,
// TLS settings (K-13) and SASL mechanism as the export transport, so a probe
// verifies the connection exports actually use.
func probeDialer(tlsCfg TLSConfig, transport *kafka.Transport) *kafka.Dialer {
	dialer := &kafka.Dialer{
		Timeout:   kafka.DefaultDialer.Timeout,
		DualStack: kafka.DefaultDialer.DualStack,
		DialFunc:  netguard.DefaultDialer.DialContext,
		TLS:       tlsCfg.ClientConfig(),
	}
	if transport != nil && transport.SASL != nil {
		dialer.SASLMechanism = transport.SASL
	}

	return dialer
}
