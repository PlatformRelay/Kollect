// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestTestConnection_missingDatabaseRef(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{},
	}, nil, nil)
	if err == nil {
		t.Fatal("expected error when databaseRef is missing")
	}
}

func TestTestConnection_invalidDSN(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}, map[string][]byte{"dsn": []byte("postgres://invalid-host:1/bad")}, nil)
	if err == nil {
		t.Fatal("expected connect error for unreachable host")
	}
}
