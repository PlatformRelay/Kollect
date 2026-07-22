// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"fmt"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestConnection pings PostgreSQL using databaseRef credentials.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) error {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return err
	}

	pool, err := newGuardedPool(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("postgres connect: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}

	return nil
}
