// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/collect"
)

func TestToMergeRows_marshalError(t *testing.T) {
	t.Parallel()

	ch := make(chan int)
	_, err := toMergeRows([]collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
		Namespace:  "team-a",
		Attributes: map[string]any{"bad": ch},
	}}, "team-a", "apps", "prod-a")
	if err == nil || !strings.Contains(err.Error(), "marshal item") {
		t.Fatalf("toMergeRows() error = %v, want marshal error", err)
	}
}

func TestToMergeRows_UsesNamespaceFallbackAndTrimsWhitespace(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{
			UID:        "uid-1",
			TargetName: "deployments",
			Namespace:  " ",
			Name:       "api",
		},
	}

	rows, err := toMergeRows(items, "team-a", "apps", "prod-a")
	if err != nil {
		t.Fatalf("toMergeRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(rows))
	}
	if rows[0].ResourceNamespace != "team-a" {
		t.Fatalf("resource namespace = %q, want team-a", rows[0].ResourceNamespace)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rows[0].PayloadJSON), &payload); err != nil {
		t.Fatalf("payload json decode: %v", err)
	}
	if payload["name"] != "api" {
		t.Fatalf("payload name = %v, want api", payload["name"])
	}
}

func TestMergeSourceRowsSQL_EscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	sql := mergeSourceRowsSQL([]mergeRow{
		{
			InventoryNamespace: "team-a",
			InventoryName:      "apps",
			Cluster:            "prod'a",
			TargetName:         "deploy'ments",
			SourceUID:          "uid-1",
			ResourceNamespace:  "workloads",
			PayloadJSON:        `{"name":"o'reilly"}`,
		},
	})

	if !strings.Contains(sql, "'prod''a'") {
		t.Fatalf("sql missing escaped cluster literal: %s", sql)
	}
	if !strings.Contains(sql, "'deploy''ments'") {
		t.Fatalf("sql missing escaped target literal: %s", sql)
	}
	if !strings.Contains(sql, "UNION ALL") && !strings.Contains(sql, "SELECT") {
		t.Fatalf("sql does not contain select rows: %s", sql)
	}
}

func TestUsingEmulator_RespectsTrimmedEnvVar(t *testing.T) {
	t.Parallel()

	orig := os.Getenv("BIGQUERY_EMULATOR_HOST")
	t.Cleanup(func() {
		_ = os.Setenv("BIGQUERY_EMULATOR_HOST", orig)
	})

	if err := os.Setenv("BIGQUERY_EMULATOR_HOST", "   "); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if usingEmulator() {
		t.Fatal("usingEmulator() = true for blank env")
	}

	if err := os.Setenv("BIGQUERY_EMULATOR_HOST", "localhost:9050"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if !usingEmulator() {
		t.Fatal("usingEmulator() = false, want true")
	}
}
