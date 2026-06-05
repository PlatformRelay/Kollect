// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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
