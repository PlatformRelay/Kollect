// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestMarshalExportEnvelopeGolden(t *testing.T) {
	t.Parallel()

	items := []Item{{
		TargetNamespace: "team-a",
		TargetName:      "deployments",
		Namespace:       "team-a",
		Name:            "api",
		Group:           "apps",
		Version:         "v1",
		Kind:            "Deployment",
		UID:             "uid-abc",
		Attributes:      map[string]any{"replicas": float64(2)},
	}}

	exportedAt, err := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	if err != nil {
		t.Fatalf("Parse exportedAt: %v", err)
	}

	got, err := MarshalExportEnvelope(items, ExportMetadata{
		Generation: 3,
		Cluster:    "prod-west",
		ExportedAt: exportedAt,
	})
	if err != nil {
		t.Fatalf("MarshalExportEnvelope: %v", err)
	}

	var env ExportEnvelope
	if unmarshalErr := json.Unmarshal(got, &env); unmarshalErr != nil {
		t.Fatalf("Unmarshal envelope: %v", unmarshalErr)
	}

	if env.SchemaVersion != ExportSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", env.SchemaVersion, ExportSchemaVersion)
	}

	if env.ItemCount != 1 || len(env.Items) != 1 {
		t.Fatalf("item count = %d / len(items) = %d", env.ItemCount, len(env.Items))
	}

	if env.Checksum == "" {
		t.Fatal("expected non-empty checksum")
	}

	wantPath := filepath.Join(repoRoot(t), "test", "schema", "golden", "export-envelope.json")
	wantBytes, readErr := os.ReadFile(filepath.Clean(wantPath)) //nolint:gosec // test reads repo golden fixture
	if readErr != nil {
		t.Fatalf("ReadFile golden: %v", readErr)
	}

	var want ExportEnvelope
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("Unmarshal golden: %v", err)
	}

	if env.SchemaVersion != want.SchemaVersion ||
		env.Generation != want.Generation ||
		env.ItemCount != want.ItemCount ||
		env.ExportedAt != want.ExportedAt ||
		env.Cluster != want.Cluster {
		t.Fatalf("metadata mismatch: got=%+v want=%+v", env, want)
	}

	if len(env.Items) != len(want.Items) {
		t.Fatalf("items len = %d, want %d", len(env.Items), len(want.Items))
	}

	if env.Items[0].UID != want.Items[0].UID || env.Items[0].Name != want.Items[0].Name {
		t.Fatalf("item identity mismatch: got=%+v want=%+v", env.Items[0], want.Items[0])
	}
}

func TestItemsFromExportPayload_envelopeAndLegacy(t *testing.T) {
	t.Parallel()

	items := []Item{{
		TargetNamespace: "ns",
		TargetName:      "t",
		UID:             "u1",
		Namespace:       "ns",
		Name:            "app",
		Version:         "v1",
		Kind:            "Pod",
	}}

	envPayload, err := MarshalExportEnvelope(items, ExportMetadata{})
	if err != nil {
		t.Fatalf("MarshalExportEnvelope: %v", err)
	}

	fromEnv, err := ItemsFromExportPayload(envPayload)
	if err != nil {
		t.Fatalf("from envelope: %v", err)
	}

	if len(fromEnv) != 1 || fromEnv[0].UID != "u1" {
		t.Fatalf("from envelope = %#v", fromEnv)
	}

	legacy, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}

	fromLegacy, err := ItemsFromExportPayload(legacy)
	if err != nil {
		t.Fatalf("from legacy: %v", err)
	}

	if len(fromLegacy) != 1 || fromLegacy[0].UID != "u1" {
		t.Fatalf("from legacy = %#v", fromLegacy)
	}
}

func TestItemsFingerprint_stable(t *testing.T) {
	t.Parallel()

	items := []Item{{UID: "a", Namespace: "ns", Name: "x", Version: "v1", Kind: "Pod"}}
	a, err := ItemsFingerprint(items)
	if err != nil {
		t.Fatalf("ItemsFingerprint: %v", err)
	}

	b, err := ItemsFingerprint(items)
	if err != nil {
		t.Fatalf("ItemsFingerprint: %v", err)
	}

	if a != b || a == "" {
		t.Fatalf("fingerprints = %q / %q", a, b)
	}
}

func TestItemsFingerprint_orderIndependent(t *testing.T) {
	t.Parallel()

	first := []Item{
		{UID: "uid-1", Namespace: "ns", Name: "export-cm-1", Version: "v1", Kind: "ConfigMap"},
		{UID: "uid-2", Namespace: "ns", Name: "export-cm-2", Version: "v1", Kind: "ConfigMap"},
	}
	second := []Item{first[1], first[0]}

	fp1, err := ItemsFingerprint(first)
	if err != nil {
		t.Fatalf("ItemsFingerprint: %v", err)
	}

	fp2, err := ItemsFingerprint(second)
	if err != nil {
		t.Fatalf("ItemsFingerprint: %v", err)
	}

	if fp1 != fp2 || fp1 == "" {
		t.Fatalf("fingerprints = %q / %q", fp1, fp2)
	}
}

func TestItemsFromExportPayloadEdgeCases(t *testing.T) {
	t.Parallel()

	if items, err := ItemsFromExportPayload(nil); err != nil || items != nil {
		t.Fatalf("nil payload = %#v err=%v", items, err)
	}

	if items, err := ItemsFromExportPayload([]byte("[]")); err != nil || items != nil {
		t.Fatalf("empty array = %#v err=%v", items, err)
	}

	_, err := ItemsFromExportPayload([]byte(`{"schemaVersion":"kollect.dev/v99","items":[]}`))
	if err == nil {
		t.Fatal("expected unsupported schemaVersion error")
	}
}

func TestMarshalExportEnvelopeNilItemsUsesNow(t *testing.T) {
	t.Parallel()

	got, err := MarshalExportEnvelope(nil, ExportMetadata{Cluster: "spoke-a"})
	if err != nil {
		t.Fatal(err)
	}

	var env ExportEnvelope
	if err := json.Unmarshal(got, &env); err != nil {
		t.Fatal(err)
	}
	if env.ItemCount != 0 || env.ExportedAt == "" {
		t.Fatalf("env = %+v", env)
	}
}
