//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/integrationtest"
	"github.com/konih/kollect/internal/sink"
	postgressink "github.com/konih/kollect/internal/sink/postgres"
)

func TestHubHTTPIngestPostgresExportRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgContainer, err := postgres.Run(ctx,
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
	t.Cleanup(func() { _ = pgContainer.Terminate(context.Background()) })

	pgConn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForPostgresRollback(ctx, pgConn); err != nil {
		t.Fatal(err)
	}

	schemaBackend, err := postgressink.NewBackend(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-bootstrap"},
			Table:       "inventory_items",
			Schema:      "public",
		},
	}, map[string][]byte{"dsn": []byte(pgConn)})
	if err != nil {
		t.Fatalf("bootstrap postgres schema: %v", err)
	}
	schemaBackend.Close()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pgSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "hub-postgres", Namespace: "platform"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:    "postgres",
			Cluster: "hub",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
				Schema:      "public",
			},
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "platform"},
		Data:       map[string][]byte{"dsn": []byte("postgres://nobody:nopass@127.0.0.1:1/nope?sslmode=disable")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pgSink, pgSecret).
		Build()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "spoke-a",
		TargetName:      "inv",
		Namespace:       "apps",
		Name:            "existing",
		UID:             "uid-old",
		Version:         "v1",
		Kind:            "Deployment",
	})

	srv := &IngestServer{
		Enabled: true,
		Auth:    IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:  NewMerger(store),
		Exporter: &Exporter{
			Store:    store,
			Client:   cl,
			Registry: sink.NewRegistry(),
			Config: ExportConfig{
				ExportNamespace: "platform",
				SinkRefs:        []string{"hub-postgres"},
			},
		},
	}

	report := SpokeReport{
		APIVersion:    export.WireAPIVersion,
		SchemaVersion: export.SchemaVersion,
		Cluster:       "spoke-a",
		InventoryRef:  InventoryRef{Namespace: "team-a", Name: "inv"},
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "t",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-new",
			Version:         "v1",
			Kind:            "Deployment",
		}},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	snap := store.SnapshotTarget("spoke-a", "inv")
	if len(snap) != 1 || snap[0].UID != "uid-old" {
		t.Fatalf("rollback snapshot = %+v, want prior uid-old item only", snap)
	}

	pool, err := pgxpool.New(ctx, pgConn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE source_uid IN ($1, $2)
`, "uid-old", "uid-new").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("postgres rows after failed ingest = %d, want 0 (no partial export persisted)", count)
	}
}

func waitForPostgresRollback(ctx context.Context, connStr string) error {
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

	return lastErr
}
