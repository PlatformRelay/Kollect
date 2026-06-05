//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"

	"github.com/konih/kollect/internal/integrationtest"
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
			t.Skipf("docker not available: %v", err)
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

	backend, err := NewBackend(spec, map[string][]byte{"dsn": []byte(connStr)})
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
