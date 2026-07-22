// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

func newGuardedPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres DSN: %w", err)
	}
	poolCfg.ConnConfig.DialFunc = netguard.DefaultDialer.DialContext

	return pgxpool.NewWithConfig(ctx, poolCfg)
}
