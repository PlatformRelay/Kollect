// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink/cap"
)

// TypeName is the KollectSink.spec.type value for Postgres sinks.
const TypeName = "postgres"

const typeName = TypeName

const connectTimeout = 30 * time.Second

// Backend upserts inventory rows into PostgreSQL.
type Backend struct {
	cfg  Config
	pool *pgxpool.Pool
}

// NewBackend constructs a postgres sink backend.
func NewBackend(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return nil, err
	}

	connectCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, connectTimeout)
		defer cancel()
	}

	pool, err := pgxpool.New(connectCtx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	b := &Backend{cfg: cfg, pool: pool}
	// ensureTable runs once when the backend is constructed; pooled backends reuse the same
	// instance so DDL is not repeated on every export (PERF-02).
	if err := b.ensureTable(connectCtx); err != nil {
		pool.Close()

		return nil, err
	}

	return b, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Capabilities reports relational upsert with delete reconciliation (ADR-0401).
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.RelationalStore()
}

// Close releases the connection pool.
func (b *Backend) Close() {
	if b.pool != nil {
		b.pool.Close()
	}
}

// Export upserts each inventory item keyed by inventory, target, and source UID,
// then deletes rows absent from the current snapshot (ADR-0401 delete reconciliation).
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	items, err := collect.ItemsFromExportPayload(payload)
	if err != nil {
		return fmt.Errorf("postgres export: decode payload: %w", err)
	}

	invNS, invName := inventoryFromObjectPath(objectPath)
	exportedAt := time.Now().UTC()
	qualifiedTable := pgxQuoteIdent(b.cfg.Schema) + "." + pgxQuoteIdent(b.cfg.Table)

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres export: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := b.upsertItems(ctx, tx, qualifiedTable, invNS, invName, b.cfg.Cluster, items, exportedAt); err != nil {
		return err
	}

	if err := deleteStaleRows(ctx, tx, qualifiedTable, invNS, invName, b.cfg.Cluster, items); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres export: commit tx: %w", err)
	}

	return nil
}

func deleteStaleRows(
	ctx context.Context,
	tx pgx.Tx,
	qualifiedTable string,
	invNS, invName, cluster string,
	items []collect.Item,
) error {
	plan := buildStaleDeletePlan(items)
	if plan.deleteAll {
		_, err := tx.Exec(ctx, fmt.Sprintf(`
DELETE FROM %s
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, qualifiedTable), invNS, invName, cluster)
		if err != nil {
			return fmt.Errorf("postgres delete all: %w", err)
		}

		return nil
	}

	_, err := tx.Exec(ctx, fmt.Sprintf(`
DELETE FROM %s AS t
WHERE t.inventory_namespace = $1
  AND t.inventory_name = $2
  AND t.cluster = $3
  AND NOT EXISTS (
    SELECT 1
    FROM unnest($4::text[], $5::text[]) AS s(target_name, source_uid)
    WHERE s.target_name = t.target_name AND s.source_uid = t.source_uid
  )
`, qualifiedTable), invNS, invName, cluster, plan.targetNames, plan.sourceUIDs)
	if err != nil {
		return fmt.Errorf("postgres delete stale: %w", err)
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
