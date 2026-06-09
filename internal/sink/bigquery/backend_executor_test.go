// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"

	"github.com/konih/kollect/internal/collect"
)

type recordedQueryCall struct {
	statement string
	params    []bigquery.QueryParameter
	location  string
}

type fakeQueryExecutor struct {
	calls []recordedQueryCall
	errs  []error
}

func (f *fakeQueryExecutor) Execute(
	_ context.Context,
	statement string,
	params []bigquery.QueryParameter,
	location string,
) error {
	f.calls = append(f.calls, recordedQueryCall{
		statement: statement,
		params:    append([]bigquery.QueryParameter(nil), params...),
		location:  location,
	})
	idx := len(f.calls) - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return f.errs[idx]
	}

	return nil
}

func TestRunDeleteAll_DelegatesToExecutor(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items", Location: "EU"},
		executor: exec,
	}

	if err := b.runDeleteAll(t.Context(), "team-a", "apps", "cluster-a"); err != nil {
		t.Fatalf("runDeleteAll() error = %v", err)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0].statement, "DELETE FROM `proj.inventory.items`") {
		t.Fatalf("statement = %q", exec.calls[0].statement)
	}
	if exec.calls[0].location != "EU" {
		t.Fatalf("location = %q, want EU", exec.calls[0].location)
	}
}

func TestRunMergeUpsert_WrapsExecutorErrors(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{errs: []error{errors.New("run failed")}}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items"},
		executor: exec,
	}

	err := b.runMergeUpsert(t.Context(), []mergeRow{{
		InventoryNamespace: "team-a",
		InventoryName:      "apps",
		Cluster:            "cluster-a",
		TargetName:         "deployments",
		SourceUID:          "uid-1",
		ResourceNamespace:  "team-a",
		PayloadJSON:        `{"name":"api"}`,
	}})
	if err == nil || !strings.Contains(err.Error(), "bigquery merge upsert") {
		t.Fatalf("runMergeUpsert() error = %v, want merge upsert wrapper", err)
	}
}

func TestExport_EmptySnapshotDeletesAll(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items", Cluster: "cluster-a"},
		executor: exec,
	}

	if err := b.Export(t.Context(), mustEnvelope(t, nil), "inventory/team-a/apps.json"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0].statement, "DELETE FROM `proj.inventory.items`") {
		t.Fatalf("statement = %q", exec.calls[0].statement)
	}
}

func TestExport_NonEmptySnapshotMergesThenDeletesStale(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items", Cluster: "cluster-a"},
		executor: exec,
	}

	payload := mustEnvelope(t, []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
		Namespace:  "team-a",
		Name:       "api",
		Kind:       "Deployment",
		Attributes: map[string]any{"replicas": 2},
	}})
	if err := b.Export(t.Context(), payload, "inventory/team-a/apps.json"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (merge + delete stale)", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0].statement, "MERGE `proj.inventory.items` AS t") {
		t.Fatalf("first statement = %q", exec.calls[0].statement)
	}
	if !strings.Contains(exec.calls[1].statement, "DELETE FROM `proj.inventory.items` AS t") {
		t.Fatalf("second statement = %q", exec.calls[1].statement)
	}
}

func TestExport_EmulatorModeUsesReplaceStrategy(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "localhost:9050")

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items", Cluster: "cluster-a"},
		executor: exec,
	}

	payload := mustEnvelope(t, []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
		Namespace:  "team-a",
		Name:       "api",
		Kind:       "Deployment",
		Attributes: map[string]any{"replicas": 2},
	}})
	if err := b.Export(t.Context(), payload, "inventory/team-a/apps.json"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (delete + insert)", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0].statement, "DELETE FROM `proj.inventory.items`") {
		t.Fatalf("first statement = %q", exec.calls[0].statement)
	}
	if !strings.Contains(exec.calls[1].statement, "INSERT INTO `proj.inventory.items`") {
		t.Fatalf("second statement = %q", exec.calls[1].statement)
	}
}

func TestExport_InvalidPayloadReturnsDecodeError(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg: Config{Project: "proj", Dataset: "inventory", Table: "items"},
		executor: exec,
	}
	err := b.Export(t.Context(), []byte(`{"schemaVersion":"kollect.dev/v99","items":[]}`), "inventory/team-a/apps.json")
	if err == nil || !strings.Contains(err.Error(), "decode payload") {
		t.Fatalf("Export() error = %v, want decode error", err)
	}
}

func mustEnvelope(t *testing.T, items []collect.Item) []byte {
	t.Helper()

	out, err := collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: 1,
		Cluster:    "cluster-a",
		ExportedAt: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("MarshalExportEnvelope() error = %v", err)
	}

	return out
}
