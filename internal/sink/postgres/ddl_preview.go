// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"fmt"
	"strings"
)

const (
	defaultSchema = "public"
	defaultTable  = "inventory_items"
)

// ExpectedCreateTableDDL returns the CREATE TABLE statement used in ensure provisioning mode (ADR-0416).
func ExpectedCreateTableDDL(schema, table string) string {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		schema = defaultSchema
	}

	table = strings.TrimSpace(table)
	if table == "" {
		table = defaultTable
	}

	qualified := pgxQuoteIdent(schema) + "." + pgxQuoteIdent(table)

	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  inventory_namespace TEXT NOT NULL,
  inventory_name TEXT NOT NULL,
  target_name TEXT NOT NULL,
  source_uid TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  resource_namespace TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  exported_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (inventory_namespace, inventory_name, target_name, source_uid)
)`, qualified)
}
