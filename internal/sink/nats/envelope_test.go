// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/konih/kollect/internal/export"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestMarshalEventEnvelopeGolden(t *testing.T) {
	t.Parallel()

	payload := []byte(`[{"namespace":"apps","uid":"uid-1"}]`)
	at, err := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	if err != nil {
		t.Fatalf("Parse timestamp: %v", err)
	}

	got, err := marshalEventEnvelope("prod-west", "team-a", payload, at)
	if err != nil {
		t.Fatalf("marshalEventEnvelope: %v", err)
	}

	var env EventEnvelope
	if unmarshalErr := json.Unmarshal(got, &env); unmarshalErr != nil {
		t.Fatalf("Unmarshal envelope: %v", unmarshalErr)
	}

	if env.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", env.SchemaVersion, export.SchemaVersion)
	}

	if env.Cluster != "prod-west" || env.Namespace != "team-a" {
		t.Fatalf("metadata = %+v", env)
	}

	if string(env.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", env.Payload, payload)
	}

	wantPath := filepath.Join(repoRoot(t), "test", "schema", "golden", "nats-event-envelope.json")
	wantBytes, readErr := os.ReadFile(filepath.Clean(wantPath)) //nolint:gosec // test reads repo golden fixture
	if readErr != nil {
		t.Fatalf("ReadFile golden: %v", readErr)
	}

	var want EventEnvelope
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("Unmarshal golden: %v", err)
	}

	if env.SchemaVersion != want.SchemaVersion ||
		env.Timestamp != want.Timestamp ||
		env.Cluster != want.Cluster ||
		env.Namespace != want.Namespace ||
		string(env.Payload) != string(want.Payload) {
		t.Fatalf("envelope mismatch:\ngot  = %+v\nwant = %+v", env, want)
	}
}
