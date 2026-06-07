// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/konih/kollect/internal/collect"
)

const bulkUpsertThreshold = 32

func (b *Backend) upsertItems(
	ctx context.Context,
	tx pgx.Tx,
	qualifiedTable string,
	invNS, invName, cluster string,
	items []collect.Item,
	exportedAt time.Time,
) error {
	if len(items) >= bulkUpsertThreshold {
		return b.bulkUpsertItems(ctx, tx, qualifiedTable, invNS, invName, cluster, items, exportedAt)
	}

	return b.rowUpsertItems(ctx, tx, qualifiedTable, invNS, invName, cluster, items, exportedAt)
}

func (b *Backend) rowUpsertItems(
	ctx context.Context,
	tx pgx.Tx,
	qualifiedTable string,
	invNS, invName, cluster string,
	items []collect.Item,
	exportedAt time.Time,
) error {
	for _, item := range items {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("postgres export: marshal item: %w", err)
		}

		resourceNS := item.Namespace
		if resourceNS == "" {
			resourceNS = invNS
		}

		_, err = tx.Exec(ctx, fmt.Sprintf(`
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
			cluster,
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

func (b *Backend) bulkUpsertItems(
	ctx context.Context,
	tx pgx.Tx,
	qualifiedTable string,
	invNS, invName, cluster string,
	items []collect.Item,
	exportedAt time.Time,
) error {
	_, err := tx.Exec(ctx, `
CREATE TEMP TABLE kollect_export_staging (
  inventory_namespace TEXT NOT NULL,
  inventory_name TEXT NOT NULL,
  target_name TEXT NOT NULL,
  source_uid TEXT NOT NULL,
  cluster TEXT NOT NULL,
  resource_namespace TEXT NOT NULL,
  payload JSONB NOT NULL,
  exported_at TIMESTAMPTZ NOT NULL
) ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("postgres bulk upsert: create staging: %w", err)
	}

	rows := make([][]any, len(items))
	for i, item := range items {
		itemJSON, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			return fmt.Errorf("postgres bulk upsert: marshal item: %w", marshalErr)
		}

		resourceNS := item.Namespace
		if resourceNS == "" {
			resourceNS = invNS
		}

		rows[i] = []any{
			invNS, invName, item.TargetName, item.UID, cluster, resourceNS, string(itemJSON), exportedAt,
		}
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"kollect_export_staging"},
		[]string{
			"inventory_namespace", "inventory_name", "target_name", "source_uid",
			"cluster", "resource_namespace", "payload", "exported_at",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("postgres bulk upsert: copy: %w", err)
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s (
  inventory_namespace, inventory_name, target_name, source_uid,
  cluster, resource_namespace, payload, exported_at
)
SELECT
  inventory_namespace, inventory_name, target_name, source_uid,
  cluster, resource_namespace, payload, exported_at
FROM kollect_export_staging
ON CONFLICT (inventory_namespace, inventory_name, target_name, source_uid)
DO UPDATE SET payload = EXCLUDED.payload, exported_at = EXCLUDED.exported_at,
  cluster = EXCLUDED.cluster, resource_namespace = EXCLUDED.resource_namespace
`, qualifiedTable))
	if err != nil {
		return fmt.Errorf("postgres bulk upsert: merge: %w", err)
	}

	return nil
}
