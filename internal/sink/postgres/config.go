// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/secretkv"
)

// Config holds resolved PostgreSQL sink settings.
type Config struct {
	DSN     string
	Schema  string
	Table   string
	Cluster string
}

// ConfigFromSpec validates spec and secret data for a postgres sink.
func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (Config, error) {
	if spec.Type != typeName {
		return Config{}, fmt.Errorf("expected postgres sink, got %q", spec.Type)
	}

	if spec.Postgres == nil {
		return Config{}, fmt.Errorf("postgres sink requires spec.postgres")
	}

	pg := spec.Postgres
	if pg.DatabaseRef == nil || pg.DatabaseRef.Name == "" {
		return Config{}, fmt.Errorf("postgres sink requires spec.postgres.databaseRef")
	}

	table := strings.TrimSpace(pg.Table)
	if table == "" {
		return Config{}, fmt.Errorf("postgres sink requires spec.postgres.table")
	}

	schema := strings.TrimSpace(pg.Schema)
	if schema == "" {
		schema = defaultSchema
	}

	dsn, err := dsnFromSecret(databaseSecret)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DSN:     dsn,
		Schema:  schema,
		Table:   table,
		Cluster: strings.TrimSpace(spec.Cluster),
	}, nil
}

func dsnFromSecret(data map[string][]byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("postgres databaseRef secret is empty")
	}

	if dsn, ok := secretkv.FirstValue(data, "dsn", "url", "connectionString", "DATABASE_URL"); ok {
		return dsn, nil
	}

	return "", fmt.Errorf("postgres secret must contain dsn or url key")
}
