// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

const typeName = "postgres"

// Backend upserts inventory rows into PostgreSQL.
type Backend struct {
	cfg  Config
	pool *pgxpool.Pool
}

// NewBackend constructs a postgres sink backend.
func NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	b := &Backend{cfg: cfg, pool: pool}
	if err := b.ensureTable(context.Background()); err != nil {
		pool.Close()

		return nil, err
	}

	return b, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Close releases the connection pool.
func (b *Backend) Close() {
	if b.pool != nil {
		b.pool.Close()
	}
}

// Export upserts each inventory item keyed by inventory, target, and source UID (ADR COORDINATION).
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	if len(payload) == 0 {
		return fmt.Errorf("postgres export: empty payload")
	}

	var items []collect.Item
	if err := json.Unmarshal(payload, &items); err != nil {
		return fmt.Errorf("postgres export: decode payload: %w", err)
	}

	invNS, invName := inventoryFromObjectPath(objectPath)
	exportedAt := time.Now().UTC()
	qualifiedTable := pgxQuoteIdent(b.cfg.Schema) + "." + pgxQuoteIdent(b.cfg.Table)

	for _, item := range items {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("postgres export: marshal item: %w", err)
		}

		resourceNS := item.Namespace
		if resourceNS == "" {
			resourceNS = invNS
		}

		_, err = b.pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s (
  inventory_namespace, inventory_name, target_name, source_uid,
  cluster, resource_namespace, payload, exported_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
ON CONFLICT (inventory_namespace, inventory_name, target_name, source_uid)
DO UPDATE SET payload = EXCLUDED.payload, exported_at = EXCLUDED.exported_at,
  cluster = EXCLUDED.cluster, resource_namespace = EXCLUDED.resource_namespace
`, qualifiedTable),
			invNS,
			invName,
			item.TargetName,
			item.UID,
			b.cfg.Cluster,
			resourceNS,
			string(itemJSON),
			exportedAt,
		)
		if err != nil {
			return fmt.Errorf("postgres upsert: %w", err)
		}
	}

	return nil
}

func (b *Backend) ensureTable(ctx context.Context) error {
	qualifiedTable := pgxQuoteIdent(b.cfg.Schema) + "." + pgxQuoteIdent(b.cfg.Table)

	_, err := b.pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  inventory_namespace TEXT NOT NULL,
  inventory_name TEXT NOT NULL,
  target_name TEXT NOT NULL,
  source_uid TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  resource_namespace TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  exported_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (inventory_namespace, inventory_name, target_name, source_uid)
)
`, qualifiedTable))

	return err
}

func inventoryFromObjectPath(objectPath string) (namespace, name string) {
	objectPath = strings.TrimPrefix(strings.TrimSpace(objectPath), "inventory/")
	parts := strings.Split(objectPath, "/")
	if len(parts) >= 2 {
		return parts[0], strings.TrimSuffix(parts[1], ".json")
	}

	if len(parts) == 1 && parts[0] != "" {
		return parts[0], ""
	}

	return "", ""
}

func pgxQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
