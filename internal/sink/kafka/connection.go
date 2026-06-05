// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TestConnection requests broker metadata to verify reachability.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
) error {
	cfg, err := ConfigFromSpec(spec, secretData)
	if err != nil {
		return err
	}

	transport, err := dialTransport(cfg)
	if err != nil {
		return err
	}

	dialer := &kafka.Dialer{
		Timeout:   kafka.DefaultDialer.Timeout,
		DualStack: kafka.DefaultDialer.DualStack,
	}
	if transport != nil && transport.SASL != nil {
		dialer.SASLMechanism = transport.SASL
	}

	conn, err := dialer.DialContext(ctx, "tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("kafka dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Brokers(); err != nil {
		return fmt.Errorf("kafka metadata: %w", err)
	}

	return nil
}
