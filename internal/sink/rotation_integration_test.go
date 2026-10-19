//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/integrationtest"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// rotationFakeS3 answers HeadBucket and PutObject for bucket "b" and counts the puts.
type rotationFakeS3 struct {
	puts chan string
}

func (f *rotationFakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodHead && r.URL.Path == "/b":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut:
		select {
		case f.puts <- r.URL.Path:
		default:
		}
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// SCR-1 "Rotated CA is trusted by the next export": the pooled backend built from CA X
// fails the handshake against a server signed by CA Y; after the caSecretRef Secret is
// rotated to CA Y, the next export succeeds on a rebuilt backend.
func TestS3Export_caRotation(t *testing.T) {
	enableRotationPool(t)

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")

	fake := &rotationFakeS3{puts: make(chan string, 4)}
	srv := httptest.NewUnstartedServer(fake)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{caY.Server(t)}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	c := rotationClient(t,
		rotationSecret(rotationSinkNS, "ca", "uid-ca", map[string][]byte{"ca.crt": caX.PEM}),
		rotationSecret(rotationSinkNS, "s3", "uid-s3", map[string][]byte{
			"accessKeyID": []byte("AKIDTEST"), "secretAccessKey": []byte("sentinel-key"),
		}),
	)
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:      "s3",
		Endpoint:  srv.URL + "/b",
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "s3"},
		TLS:       &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"}},
	}
	reg := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exportOnce := func() error {
		backend, release, err := acquireBackend(ctx, c, reg, rotationSinkNS, "s3-ca", "uid-s3-ca", spec)
		if err != nil {
			return err
		}
		defer release()

		return backend.Export(ctx, []byte(`{"items":[]}`), "inventory/default/inv.json")
	}

	err := exportOnce()
	var verr *tls.CertificateVerificationError
	if !errors.As(err, &verr) {
		t.Fatalf("export trusting CA X: err = %v, want a certificate verification error", err)
	}

	setSecretData(t, c, rotationSinkNS, "ca", map[string][]byte{"ca.crt": caY.PEM}, nil)

	if err := exportOnce(); err != nil {
		t.Fatalf("export after rotating the CA Secret to CA Y: %v", err)
	}
	select {
	case path := <-fake.puts:
		if path != "/b/inventory/default/inv.json" {
			t.Fatalf("server recorded PUT %s", path)
		}
	default:
		t.Fatal("server recorded no PutObject after the rotation")
	}
}

// SCR-1 "Rotated database password is used by the next export": the backend built for role
// u1 is replaced once the databaseRef Secret names u2; the second export inserts as u2 and
// no u1 session of the test's backends stays open (NOLOGIN does not end open sessions).
func TestPostgresExport_passwordRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	enableRotationPool(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("inventory"),
		tcpostgres.WithUsername("kollect"),
		tcpostgres.WithPassword("kollect"),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			integrationtest.SkipDockerUnavailable(t, err)
		}
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	adminDSN, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	admin := waitForRotationPostgres(ctx, t, adminDSN)

	for _, stmt := range []string{
		`CREATE ROLE u1 LOGIN PASSWORD 'sentinel-pass-1'`,
		`CREATE ROLE u2 LOGIN PASSWORD 'sentinel-pass-2'`,
		`CREATE TABLE public.rotation_items (
  inventory_namespace TEXT NOT NULL,
  inventory_name TEXT NOT NULL,
  target_name TEXT NOT NULL,
  source_uid TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  resource_namespace TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  exported_at TIMESTAMPTZ NOT NULL,
  written_by TEXT NOT NULL DEFAULT current_user,
  PRIMARY KEY (inventory_namespace, inventory_name, target_name, source_uid)
)`,
		`GRANT USAGE, CREATE ON SCHEMA public TO u1, u2`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON public.rotation_items TO u1, u2`,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	roleDSN := func(user, password string) []byte {
		u, perr := url.Parse(adminDSN)
		if perr != nil {
			t.Fatal(perr)
		}
		u.User = url.UserPassword(user, password)

		return []byte(u.String())
	}

	c := rotationClient(t, rotationSecret(rotationSinkNS, "db", "uid-db", map[string][]byte{"dsn": roleDSN("u1", "sentinel-pass-1")}))
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "test-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "db"},
			Table:       "rotation_items",
			Schema:      "public",
		},
	}
	reg := NewRegistry()

	exportItem := func(uid string) {
		t.Helper()
		backend, release, aerr := acquireBackend(ctx, c, reg, rotationSinkNS, "pg-rotation", "uid-pg-rotation", spec)
		if aerr != nil {
			t.Fatalf("acquire for %s: %v", uid, aerr)
		}
		defer release()
		payload, merr := json.Marshal([]collect.Item{{
			TargetNamespace: "apps", TargetName: "web", Namespace: "apps", Name: "demo-" + uid,
			Version: "v1", Kind: "Deployment", UID: uid,
		}})
		if merr != nil {
			t.Fatal(merr)
		}
		if eerr := backend.Export(ctx, payload, "inventory/apps/demo.json"); eerr != nil {
			t.Fatalf("export %s: %v", uid, eerr)
		}
	}

	exportItem("uid-1")
	if got := writtenBy(ctx, t, admin, "uid-1"); got != "u1" {
		t.Fatalf("first export written by %q, want u1", got)
	}

	setSecretData(t, c, rotationSinkNS, "db", map[string][]byte{"dsn": roleDSN("u2", "sentinel-pass-2")}, nil)
	if _, err := admin.Exec(ctx, `ALTER ROLE u1 NOLOGIN`); err != nil {
		t.Fatal(err)
	}

	exportItem("uid-2")
	if got := writtenBy(ctx, t, admin, "uid-2"); got != "u2" {
		t.Fatalf("export after rotation inserted as %q, want u2 (the pool kept authenticating as u1)", got)
	}

	deadline := time.Now().Add(10 * time.Second)
	var sessions int
	for {
		if err := admin.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity WHERE usename = 'u1'`).Scan(&sessions); err != nil {
			t.Fatal(err)
		}
		if sessions == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if sessions != 0 {
		t.Fatalf("%d u1 sessions still open after the rotation; the old backend was not closed", sessions)
	}
}

func writtenBy(ctx context.Context, t *testing.T, admin *pgx.Conn, uid string) string {
	t.Helper()

	var who string
	if err := admin.QueryRow(ctx,
		`SELECT written_by FROM public.rotation_items WHERE source_uid = $1`, uid).Scan(&who); err != nil {
		t.Fatalf("row for %s: %v", uid, err)
	}

	return who
}

func waitForRotationPostgres(ctx context.Context, t *testing.T, dsn string) *pgx.Conn {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	for {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
		}
		if err == nil {
			conn, cerr := pgx.Connect(ctx, dsn)
			if cerr != nil {
				t.Fatal(cerr)
			}
			t.Cleanup(func() { _ = conn.Close(context.Background()) })

			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("postgres not ready: %v", err)
		}
		time.Sleep(time.Second)
	}
}
