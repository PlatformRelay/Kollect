// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/pathvalidate"
	"github.com/platformrelay/kollect/internal/sink/cap"
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

	pool, err := newGuardedPool(connectCtx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	b := &Backend{cfg: cfg, pool: pool}
	// Provisioning runs once when the backend is constructed; pooled backends reuse the same
	// instance so DDL is not repeated on every export (PERF-02). In provisioning.mode=existing
	// Kollect must not create destination resources: it verifies the table instead of running
	// DDL, so a least-privilege role without CREATE can still export (ADR-0416 §5).
	if err := provisionTable(connectCtx, cfg.ProvisioningMode, b.verifyTable, b.ensureTable); err != nil {
		pool.Close()

		return nil, redactedConnectError(err)
	}

	return b, nil
}

// provisionTable runs the destination-resource provisioning step for the effective mode:
// existing verifies the table, anything else ensures it. It is a free function so the branch
// selection is unit-testable without a live database.
func provisionTable(
	ctx context.Context,
	mode string,
	verify, ensure func(context.Context) error,
) error {
	if mode == kollectdevv1alpha1.ProvisioningModeExisting {
		return verify(ctx)
	}

	return ensure(ctx)
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
		return fmt.Errorf("%w: %w", ErrDecodePayloadFailed, err)
	}

	invNS, invName := pathvalidate.InventoryFromObjectPath(objectPath)
	exportedAt := time.Now().UTC()
	qualifiedTable := pgxQuoteIdent(b.cfg.Schema) + "." + pgxQuoteIdent(b.cfg.Table)

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBeginTxFailed, redactedConnectError(err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := b.upsertItems(ctx, tx, qualifiedTable, invNS, invName, b.cfg.Cluster, items, exportedAt); err != nil {
		return err
	}

	if err := deleteStaleRows(ctx, tx, qualifiedTable, invNS, invName, b.cfg.Cluster, items); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrCommitTxFailed, redactedConnectError(err))
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
			return fmt.Errorf("%w: %w", ErrDeleteAllFailed, redactedConnectError(err))
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
		return fmt.Errorf("%w: %w", ErrDeleteStaleFailed, redactedConnectError(err))
	}

	return nil
}

// verifyTable confirms the destination table exists without creating it (provisioning.mode=existing).
// It probes pg_catalog via to_regclass rather than information_schema: information_schema only
// lists relations the current role has some privilege on, so a least-privilege role with no
// grant on an existing table would be told the table does not exist. to_regclass is
// privilege-independent, so absent and unprivileged are distinguishable. to_regclass resolves
// any relation kind, so the probe also requires an ordinary or partitioned table: a view,
// index or sequence carrying the table's name counts as absent.
func (b *Backend) verifyTable(ctx context.Context) error {
	qualified := pgxQuoteIdent(b.cfg.Schema) + "." + pgxQuoteIdent(b.cfg.Table)

	var exists bool

	err := b.pool.QueryRow(ctx,
		`SELECT c.relkind IN ('r', 'p') FROM pg_catalog.pg_class c WHERE c.oid = to_regclass($1)`,
		qualified,
	).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		exists, err = false, nil
	}

	return classifyTableProbe(b.cfg.Schema, b.cfg.Table, exists, err)
}

// classifyTableProbe maps a table-existence probe result to an error: a successful probe that
// found nothing is the shaped not-found error, while a failed probe is wrapped and must not read
// as "table absent".
func classifyTableProbe(schema, table string, exists bool, err error) error {
	if err != nil {
		return fmt.Errorf("postgres verify table: %w", redactedConnectError(err))
	}

	if !exists {
		return fmt.Errorf(
			"postgres verify table: %s.%s does not exist (provisioning.mode=existing): %w",
			schema, table, ErrTableNotFound,
		)
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

func pgxQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
