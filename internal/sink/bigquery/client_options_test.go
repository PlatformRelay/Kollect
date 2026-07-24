// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"net/http"
	"testing"

	"google.golang.org/api/option"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

// clientOptions reads BIGQUERY_EMULATOR_HOST via t.Setenv, so these tests
// cannot run with t.Parallel().

func TestClientOptions_NoEmulatorNoCreds(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected guarded HTTP client option only, got %d", len(opts))
	}
}

func TestClientOptions_ProductionPathAttachesGuardedHTTPClient(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "")

	var attached *http.Client
	old := guardedHTTPClientOption
	guardedHTTPClientOption = func() option.ClientOption {
		attached = netguard.HTTPClient(0)
		return option.WithHTTPClient(attached)
	}
	t.Cleanup(func() { guardedHTTPClientOption = old })

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if attached == nil {
		t.Fatal("expected production clientOptions to attach netguard.HTTPClient")
	}
	if len(opts) != 1 {
		t.Fatalf("expected a single guarded HTTP client option, got %d", len(opts))
	}
	transport, ok := attached.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP client transport type = %T", attached.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("production BigQuery HTTP client is missing a guarded DialContext")
	}
	if transport.Proxy != nil {
		t.Fatal("production BigQuery HTTP client must disable proxy resolution")
	}
}

func TestClientOptions_EmulatorAddsEndpointOption(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "localhost:9050")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	// Endpoint + WithoutAuthentication + guarded HTTP client.
	if len(opts) != 3 {
		t.Fatalf("expected 3 emulator options, got %d", len(opts))
	}
}

func TestClientOptions_EmulatorSchemePreserved(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "https://emulator.example:9050")

	opts, err := Config{}.clientOptions(context.Background())
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("expected 3 emulator options, got %d", len(opts))
	}
}

func TestClientOptions_InvalidCredentialsJSONErrors(t *testing.T) {
	t.Setenv("BIGQUERY_EMULATOR_HOST", "")

	cfg := Config{CredentialsJSON: []byte("not-json")}
	if _, err := cfg.clientOptions(context.Background()); err == nil {
		t.Fatal("expected error for unparseable credentials.json")
	}
}
