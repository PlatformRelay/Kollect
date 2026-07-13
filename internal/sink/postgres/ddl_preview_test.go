// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"strings"
	"testing"
)

func TestExpectedCreateTableDDL_QualifiesAndQuotesIdentifiers(t *testing.T) {
	t.Parallel()

	ddl := ExpectedCreateTableDDL("analytics", "items")

	if !strings.Contains(ddl, `"analytics"."items"`) {
		t.Fatalf("DDL missing quoted qualified table, got:\n%s", ddl)
	}
	if !strings.HasPrefix(ddl, "CREATE TABLE IF NOT EXISTS ") {
		t.Fatalf("DDL missing CREATE TABLE prefix, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, "PRIMARY KEY (inventory_namespace, inventory_name, target_name, source_uid)") {
		t.Fatalf("DDL missing primary key, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, "payload JSONB NOT NULL") {
		t.Fatalf("DDL missing payload column, got:\n%s", ddl)
	}
}

func TestExpectedCreateTableDDL_DefaultsBlankSchemaAndTable(t *testing.T) {
	t.Parallel()

	ddl := ExpectedCreateTableDDL("  ", "\t")

	if !strings.Contains(ddl, `"public"."inventory_items"`) {
		t.Fatalf("DDL did not fall back to default schema/table, got:\n%s", ddl)
	}
}

func TestExpectedCreateTableDDL_EscapesEmbeddedQuotes(t *testing.T) {
	t.Parallel()

	ddl := ExpectedCreateTableDDL(`we"ird`, `ta"ble`)

	if !strings.Contains(ddl, `"we""ird"."ta""ble"`) {
		t.Fatalf("DDL did not escape embedded quotes, got:\n%s", ddl)
	}
}
