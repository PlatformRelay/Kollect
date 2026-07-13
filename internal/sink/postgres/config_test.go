// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestConfigFromSpec(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"}, nil)
	if err == nil {
		t.Fatal("expected error without postgres spec")
	}

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "prod-a",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "items",
		},
	}, map[string][]byte{"dsn": []byte("postgres://localhost/inventory")})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}

	if cfg.Schema != "public" {
		t.Fatalf("schema = %q, want public", cfg.Schema)
	}

	if cfg.Cluster != "prod-a" {
		t.Fatalf("cluster = %q, want prod-a", cfg.Cluster)
	}
}

func TestConfigFromSpec_wrongType(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "mongodb"}, nil)
	if err == nil {
		t.Fatal("expected error for wrong sink type")
	}
}

func TestConfigFromSpec_missingDatabaseRef(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{Table: "items"},
	}, map[string][]byte{"dsn": []byte("postgres://localhost/inventory")})
	if err == nil {
		t.Fatal("expected error without databaseRef")
	}
}

func TestConfigFromSpec_missingTable(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "   ",
		},
	}, map[string][]byte{"dsn": []byte("postgres://localhost/inventory")})
	if err == nil {
		t.Fatal("expected error for blank table")
	}
}

func TestConfigFromSpec_overridesSchema(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "items",
			Schema:      "analytics",
		},
	}, map[string][]byte{"url": []byte("postgres://localhost/inventory")})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}

	if cfg.Schema != "analytics" {
		t.Fatalf("schema = %q, want analytics", cfg.Schema)
	}
	if cfg.Table != "items" {
		t.Fatalf("table = %q, want items", cfg.Table)
	}
	if cfg.DSN != "postgres://localhost/inventory" {
		t.Fatalf("dsn = %q, want resolution from url key", cfg.DSN)
	}
}

func TestConfigFromSpec_missingPostgresBlock(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"}, nil)
	if err == nil {
		t.Fatal("expected error without spec.postgres")
	}
}

func TestDSNFromSecret_emptySecret(t *testing.T) {
	t.Parallel()

	if _, err := dsnFromSecret(nil); err == nil {
		t.Fatal("expected error for nil secret")
	}
	if _, err := dsnFromSecret(map[string][]byte{}); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestDSNFromSecret_recognizedKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{name: "dsn", key: "dsn"},
		{name: "url", key: "url"},
		{name: "connectionString", key: "connectionString"},
		{name: "DATABASE_URL", key: "DATABASE_URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := "postgres://host/db"
			got, err := dsnFromSecret(map[string][]byte{tt.key: []byte(want)})
			if err != nil {
				t.Fatalf("dsnFromSecret(%s): %v", tt.key, err)
			}
			if got != want {
				t.Fatalf("dsn = %q, want %q", got, want)
			}
		})
	}
}

func TestDSNFromSecret_unknownKeys(t *testing.T) {
	t.Parallel()

	if _, err := dsnFromSecret(map[string][]byte{"password": []byte("secret")}); err == nil {
		t.Fatal("expected error for secret with no recognized key")
	}
}
