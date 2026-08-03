// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// dialConnectError returns a real *pgconn.ConnectError whose text embeds the
// tenant user, database, and target host — the exact leak class SEC-01-FUP3
// closes on the runtime Export path. 127.0.0.1:1 is rejected by netguard as a
// non-globally-routable address before any egress, so it is hermetic and
// instant. The fixture is guarded: if pgx ever stops leaking the host the tests
// that consume this error would pass vacuously, so we fail loudly here instead.
func dialConnectError(t *testing.T) error {
	t.Helper()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	dsn := "postgres://" + user + ":" + pass + "@127.0.0.1:1/" + db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := newGuardedPool(ctx, dsn)
	if err != nil {
		t.Fatalf("newGuardedPool returned an unexpected error: %v", err)
	}
	defer pool.Close()

	raw := pool.Ping(ctx)
	if raw == nil {
		t.Fatal("expected a dial failure pinging an unreachable host")
	}
	if !strings.Contains(raw.Error(), "127.0.0.1") {
		t.Fatalf("fixture no longer leaks the host, revisit the test: %v", raw)
	}

	return raw
}

// exportRedactSecrets are the sensitive substrings that must never survive
// redaction on any runtime Export error path.
var exportRedactSecrets = []string{"tenantuser", "hunter2SECRET", "127.0.0.1", "tenantdb", "user="}

// TestExport_RedactsConnectErrorOnBegin drives a real lazy-reconnect
// *pgconn.ConnectError through the runtime Export path end-to-end: a Backend
// whose pool targets an unreachable host fails at b.pool.Begin, and the error
// returned to the caller (controller status/events) must carry the sentinel
// but none of the DSN host/credential tokens (SEC-01-FUP3).
func TestExport_RedactsConnectErrorOnBegin(t *testing.T) {
	t.Parallel()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	dsn := "postgres://" + user + ":" + pass + "@127.0.0.1:1/" + db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := newGuardedPool(ctx, dsn)
	if err != nil {
		t.Fatalf("newGuardedPool returned an unexpected error: %v", err)
	}
	defer pool.Close()

	b := &Backend{cfg: Config{Schema: "public", Table: "inventory"}, pool: pool}
	payload, err := json.Marshal(makeItems(1))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	got := b.Export(ctx, payload, "inventory/apps/demo.json")
	if got == nil {
		t.Fatal("Export must return a non-nil error when the pool cannot connect")
	}
	if !errors.Is(got, ErrBeginTxFailed) {
		t.Errorf("Export() error = %v, want wrapped ErrBeginTxFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestRowUpsertItems_RedactsConnectError injects a real *pgconn.ConnectError at
// the tx.Exec seam of the row-upsert path and asserts the wrapped error keeps
// its sentinel but leaks no host/credential tokens (SEC-01-FUP3).
func TestRowUpsertItems_RedactsConnectError(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeCopyTx{execErrors: []error{connErr}}
	b := &Backend{}

	got := b.rowUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(1), time.Now().UTC())
	if got == nil {
		t.Fatal("rowUpsertItems must return a non-nil error")
	}
	if !errors.Is(got, ErrUpsertFailed) {
		t.Errorf("rowUpsertItems() error = %v, want wrapped ErrUpsertFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestBulkUpsertItems_RedactsConnectErrorOnStaging injects a ConnectError at the
// staging CREATE TEMP TABLE Exec seam.
func TestBulkUpsertItems_RedactsConnectErrorOnStaging(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeCopyTx{execErrors: []error{connErr}}
	b := &Backend{}

	got := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if got == nil {
		t.Fatal("bulkUpsertItems must return a non-nil error")
	}
	if !errors.Is(got, ErrBulkUpsertCreateStagingFailed) {
		t.Errorf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertCreateStagingFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestBulkUpsertItems_RedactsConnectErrorOnCopy injects a ConnectError at the
// CopyFrom seam.
func TestBulkUpsertItems_RedactsConnectErrorOnCopy(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeCopyTx{copyErr: connErr}
	b := &Backend{}

	got := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if got == nil {
		t.Fatal("bulkUpsertItems must return a non-nil error")
	}
	if !errors.Is(got, ErrBulkUpsertCopyFailed) {
		t.Errorf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertCopyFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestBulkUpsertItems_RedactsConnectErrorOnMerge injects a ConnectError at the
// final INSERT ... SELECT merge Exec seam (second Exec call).
func TestBulkUpsertItems_RedactsConnectErrorOnMerge(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeCopyTx{execErrors: []error{nil, connErr}}
	b := &Backend{}

	got := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if got == nil {
		t.Fatal("bulkUpsertItems must return a non-nil error")
	}
	if !errors.Is(got, ErrBulkUpsertMergeFailed) {
		t.Errorf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertMergeFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestDeleteStaleRows_RedactsConnectError injects a ConnectError at the
// stale-delete tx.Exec seam.
func TestDeleteStaleRows_RedactsConnectError(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeDeleteTx{execErrors: []error{connErr}}
	items := makeItems(1)

	got := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items)
	if got == nil {
		t.Fatal("deleteStaleRows must return a non-nil error")
	}
	if !errors.Is(got, ErrDeleteStaleFailed) {
		t.Errorf("deleteStaleRows() error = %v, want wrapped ErrDeleteStaleFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}

// TestDeleteStaleRows_RedactsConnectErrorOnDeleteAll injects a ConnectError at
// the delete-all tx.Exec seam. An empty snapshot ([]byte("[]") in Export) takes
// the plan.deleteAll branch, a distinct runtime Export site (ErrDeleteAllFailed).
func TestDeleteStaleRows_RedactsConnectErrorOnDeleteAll(t *testing.T) {
	t.Parallel()

	connErr := dialConnectError(t)
	tx := &fakeDeleteTx{execErrors: []error{connErr}}

	// nil items → buildStaleDeletePlan.deleteAll == true.
	got := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", nil)
	if got == nil {
		t.Fatal("deleteStaleRows must return a non-nil error")
	}
	if !errors.Is(got, ErrDeleteAllFailed) {
		t.Errorf("deleteStaleRows() error = %v, want wrapped ErrDeleteAllFailed", got)
	}
	assertNoDSNTokens(t, got.Error(), exportRedactSecrets...)
}
