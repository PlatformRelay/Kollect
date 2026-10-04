//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"

	"github.com/platformrelay/kollect/internal/integrationtest"
)

func TestExportPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("kollect"),
		postgres.WithPassword("kollect"),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}

		t.Fatalf("start postgres: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForPostgres(ctx, connStr); err != nil {
		t.Fatal(err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "test-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"dsn": []byte(connStr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)

	items := []collect.Item{
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "demo",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 2},
		},
	}
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND source_uid = $3
`, "apps", "demo", "uid-1").Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}

	updated := items
	updated[0].Attributes = map[string]any{"replicas": 3}
	payload, err = json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export upsert: %v", err)
	}

	var replicas float64
	if err := pool.QueryRow(ctx, `
SELECT (payload->'attributes'->>'replicas')::float
FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND source_uid = $3
`, "apps", "demo", "uid-1").Scan(&replicas); err != nil {
		t.Fatal(err)
	}

	if replicas != 3 {
		t.Fatalf("replicas = %v, want 3", replicas)
	}

	// Export a snapshot with an extra row, then a reduced snapshot — stale row must be deleted (ADR-0401).
	reduced := updated
	extra := collect.Item{
		TargetNamespace: "apps",
		TargetName:      "web",
		Namespace:       "apps",
		Name:            "extra",
		Version:         "v1",
		Kind:            "Deployment",
		UID:             "uid-stale",
	}
	extraPayload, err := json.Marshal(append(items, extra))
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, extraPayload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export with extra row: %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.inventory_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("row count after extra export = %d, want 2", count)
	}

	reducedPayload, err := json.Marshal(reduced)
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, reducedPayload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export delete recon: %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.inventory_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count after delete recon = %d, want 1", count)
	}

	// Empty snapshot deletes all rows for this inventory + cluster.
	if err := backend.Export(ctx, []byte("[]"), "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export empty snapshot: %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.inventory_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("row count after empty export = %d, want 0", count)
	}
}

func TestExportPostgresBulk(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("kollect"),
		postgres.WithPassword("kollect"),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}

		t.Fatalf("start postgres: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForPostgres(ctx, connStr); err != nil {
		t.Fatal(err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "bulk-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"dsn": []byte(connStr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)

	const rowCount = 500
	items := make([]collect.Item, rowCount)
	for i := range rowCount {
		items[i] = collect.Item{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            fmt.Sprintf("demo-%04d", i),
			Version:         "v1",
			Kind:            "Deployment",
			UID:             fmt.Sprintf("uid-%04d", i),
			Attributes:      map[string]any{"replicas": i},
		}
	}

	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := backend.Export(ctx, payload, "inventory/apps/bulk.json"); err != nil {
		t.Fatalf("Export bulk: %v", err)
	}
	t.Logf("bulk export %d rows in %s", rowCount, time.Since(start))

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, "apps", "bulk", "bulk-cluster").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != rowCount {
		t.Fatalf("row count = %d, want %d", count, rowCount)
	}
}

// TestExportPostgres_idempotentDoubleExport asserts a second Export of the same
// snapshot upserts in place (no duplicate rows) and that a reduced snapshot
// reconciles stale rows via the delete plan (COV-90-S10 / Track B).
func TestExportPostgres_idempotentDoubleExport(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, connStr := startIntegrationPostgres(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "idempotent-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"dsn": []byte(connStr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)

	items := []collect.Item{
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "demo",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-idem-1",
			Attributes:      map[string]any{"replicas": 2},
		},
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "worker",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-idem-2",
			Attributes:      map[string]any{"replicas": 1},
		},
	}
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	const objectPath = "inventory/apps/idempotent.json"
	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export re-run: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, "apps", "idempotent", "idempotent-cluster").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("row count after identical re-export = %d, want 2 (no dupes)", count)
	}

	reduced := []collect.Item{items[0]}
	reducedPayload, err := json.Marshal(reduced)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, reducedPayload, objectPath); err != nil {
		t.Fatalf("Export reduced: %v", err)
	}

	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, "apps", "idempotent", "idempotent-cluster").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count after stale reconcile = %d, want 1", count)
	}
}

// TestExportPostgres_midBatchConstraintViolationRollsBack asserts that a
// mid-batch constraint violation aborts the whole export transaction — prior
// rows in the same snapshot are not committed — and the error wraps
// ErrUpsertFailed (COV-90-S10 / Track B partial-failure).
func TestExportPostgres_midBatchConstraintViolationRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, connStr := startIntegrationPostgres(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "partial-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"dsn": []byte(connStr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)

	seed := []collect.Item{{
		TargetNamespace: "apps",
		TargetName:      "web",
		Namespace:       "apps",
		Name:            "seed",
		Version:         "v1",
		Kind:            "Deployment",
		UID:             "uid-seed",
		Attributes:      map[string]any{"replicas": 1},
	}}
	seedPayload, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, seedPayload, "inventory/apps/partial.json"); err != nil {
		t.Fatalf("seed Export: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `
ALTER TABLE public.inventory_items
ADD CONSTRAINT reject_poison_uid CHECK (source_uid <> 'uid-poison')
`)
	if err != nil {
		t.Fatalf("add check constraint: %v", err)
	}

	batch := []collect.Item{
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "ok",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-ok",
			Attributes:      map[string]any{"replicas": 5},
		},
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "poison",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-poison",
			Attributes:      map[string]any{"replicas": 9},
		},
	}
	batchPayload, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}

	err = backend.Export(ctx, batchPayload, "inventory/apps/partial.json")
	if err == nil {
		t.Fatal("expected mid-batch constraint violation")
	}
	if !errors.Is(err, ErrUpsertFailed) {
		t.Fatalf("Export() error = %v, want wrapped ErrUpsertFailed", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, "apps", "partial", "partial-cluster").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count after rolled-back export = %d, want 1 (seed only)", count)
	}

	var seedUID string
	if err := pool.QueryRow(ctx, `
SELECT source_uid FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND cluster = $3
`, "apps", "partial", "partial-cluster").Scan(&seedUID); err != nil {
		t.Fatal(err)
	}
	if seedUID != "uid-seed" {
		t.Fatalf("remaining source_uid = %q, want uid-seed (atomic rollback)", seedUID)
	}
}

// TestNewBackend_authFailureFailFast asserts wrong credentials fail quickly
// without hanging on connectTimeout (COV-90-S10 / Track B auth-fail).
func TestNewBackend_authFailureFailFast(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, connStr := startIntegrationPostgres(t)

	badDSN, err := rewritePostgresPassword(connStr, "definitely-wrong-password")
	if err != nil {
		t.Fatal(err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "auth-fail-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}

	start := time.Now()
	_, err = NewBackend(ctx, spec, map[string][]byte{"dsn": []byte(badDSN)})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected auth failure for wrong password")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("auth failure took %s, want fail-fast well under connectTimeout (%s)", elapsed, connectTimeout)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "postgres") {
		t.Fatalf("auth error = %v, want postgres-classified failure", err)
	}

	// TestConnection must also reject bad credentials without hanging.
	start = time.Now()
	err = TestConnection(ctx, spec, map[string][]byte{"dsn": []byte(badDSN)})
	elapsed = time.Since(start)
	if err == nil {
		t.Fatal("expected TestConnection auth failure")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("TestConnection auth failure took %s, want fail-fast", elapsed)
	}
}

// TestNewBackend_ExistingMode drives the production provisioning.mode=existing path through
// NewBackend against a real server: the table is verified, never created, and a role without
// CREATE can still construct the backend (ADR-0416 §5).
func TestNewBackend_ExistingMode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, connStr := startIntegrationPostgres(t)

	admin, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)

	createTable := func(t *testing.T, table string) {
		t.Helper()

		if _, err := admin.Exec(ctx, fmt.Sprintf(`
CREATE TABLE public.%s (
  inventory_namespace TEXT NOT NULL,
  inventory_name TEXT NOT NULL,
  target_name TEXT NOT NULL,
  source_uid TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  resource_namespace TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  exported_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (inventory_namespace, inventory_name, target_name, source_uid)
)`, table)); err != nil {
			t.Fatalf("create table %s: %v", table, err)
		}
	}

	relkind := func(t *testing.T, table string) string {
		t.Helper()

		var kind *string
		if err := admin.QueryRow(ctx, `
SELECT c.relkind::text FROM pg_catalog.pg_class c WHERE c.oid = to_regclass($1)
`, "public."+table).Scan(&kind); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("probe %s: %v", table, err)
		}
		if kind == nil {
			return ""
		}

		return *kind
	}

	t.Run("table absent is not found and not created", func(t *testing.T) {
		const table = "existing_absent"

		_, err := NewBackend(ctx, existingModeSpec(table), map[string][]byte{"dsn": []byte(connStr)})
		if !errors.Is(err, ErrTableNotFound) {
			t.Fatalf("NewBackend() error = %v, want ErrTableNotFound", err)
		}
		if kind := relkind(t, table); kind != "" {
			t.Fatalf("existing mode created relation %s (relkind %q), want none", table, kind)
		}
	})

	t.Run("table present", func(t *testing.T) {
		const table = "existing_present"
		createTable(t, table)

		backend, err := NewBackend(ctx, existingModeSpec(table), map[string][]byte{"dsn": []byte(connStr)})
		if err != nil {
			t.Fatalf("NewBackend() error = %v, want nil", err)
		}
		backend.Close()
	})

	t.Run("role without CREATE privilege", func(t *testing.T) {
		const table = "existing_least_privilege"
		createTable(t, table)

		for _, stmt := range []string{
			`CREATE ROLE kollect_writer LOGIN PASSWORD 'writer'`,
			`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
			`GRANT USAGE ON SCHEMA public TO kollect_writer`,
			fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO kollect_writer`, table),
		} {
			if _, err := admin.Exec(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}

		var canCreate bool
		if err := admin.QueryRow(ctx,
			`SELECT has_schema_privilege('kollect_writer', 'public', 'CREATE')`,
		).Scan(&canCreate); err != nil {
			t.Fatal(err)
		}
		if canCreate {
			t.Fatal("precondition: kollect_writer must lack CREATE on schema public")
		}

		writerDSN, err := rewritePostgresUser(connStr, "kollect_writer", "writer")
		if err != nil {
			t.Fatal(err)
		}

		backend, err := NewBackend(ctx, existingModeSpec(table), map[string][]byte{"dsn": []byte(writerDSN)})
		if err != nil {
			t.Fatalf("NewBackend() as role without CREATE: error = %v, want nil", err)
		}
		backend.Close()
	})

	t.Run("view with the table name is rejected", func(t *testing.T) {
		const table = "existing_view"
		if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE VIEW public.%s AS SELECT 1 AS x`, table)); err != nil {
			t.Fatalf("create view: %v", err)
		}

		_, err := NewBackend(ctx, existingModeSpec(table), map[string][]byte{"dsn": []byte(connStr)})
		if !errors.Is(err, ErrTableNotFound) {
			t.Fatalf("NewBackend() with a view named %s: error = %v, want ErrTableNotFound", table, err)
		}
	})
}

func existingModeSpec(table string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type:         "postgres",
		Cluster:      "existing-cluster",
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       table,
			Schema:      "public",
		},
	}
}

func rewritePostgresUser(dsn, user, password string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)

	return u.String(), nil
}

func startIntegrationPostgres(t *testing.T) (context.Context, string) {
	t.Helper()

	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("kollect"),
		postgres.WithPassword("kollect"),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}

		t.Fatalf("start postgres: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForPostgres(ctx, connStr); err != nil {
		t.Fatal(err)
	}

	return ctx, connStr
}

func rewritePostgresPassword(dsn, password string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	if u.User == nil {
		return "", fmt.Errorf("dsn missing userinfo: %q", dsn)
	}
	user := u.User.Username()
	u.User = url.UserPassword(user, password)

	return u.String(), nil
}

func waitForPostgres(ctx context.Context, connStr string) error {
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)

			continue
		}

		lastErr = pool.Ping(ctx)
		pool.Close()

		if lastErr == nil {
			return nil
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("postgres not ready: %w", lastErr)
}
