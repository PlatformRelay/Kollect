// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"testing"
)

// clientOptions reads BIGQUERY_EMULATOR_HOST via t.Setenv, so these tests
// cannot run with t.Parallel().

func TestClientOptions_NoEmulatorNoCreds(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected no options without emulator/creds, got %d", len(opts))
	}
}

func TestClientOptions_EmulatorAddsEndpointOption(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "localhost:9050")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	// Endpoint + WithoutAuthentication.
	if len(opts) != 2 {
		t.Fatalf("expected 2 emulator options, got %d", len(opts))
	}
}

func TestClientOptions_EmulatorSchemePreserved(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "https://emulator.example:9050")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 emulator options, got %d", len(opts))
	}
}

func TestClientOptions_InvalidCredentialsJSONErrors(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "")

	cfg := Config{CredentialsJSON: []byte("not-json")}
	if _, err := cfg.clientOptions(context.Background()); err == nil {
		t.Fatal("expected error for unparseable credentials.json")
	}
}
