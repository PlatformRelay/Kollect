// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestHelmReleaseSummaryProfile_contract(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("testdata", "flux_helmrelease.json")
	//nolint:gosec // G304: fixture path is fixed under package testdata
	raw, readErr := os.ReadFile(fixture)
	if readErr != nil {
		t.Fatalf("read fixture: %v", readErr)
	}

	var obj unstructured.Unstructured
	if unmarshalErr := json.Unmarshal(raw, &obj.Object); unmarshalErr != nil {
		t.Fatalf("decode fixture: %v", unmarshalErr)
	}

	history, found, err := unstructured.NestedSlice(obj.Object, "status", "history")
	if err != nil || !found || len(history) < 2 {
		t.Fatalf("status.history: found=%v len=%d err=%v", found, len(history), err)
	}

	newest, ok := history[0].(map[string]any)
	if !ok {
		t.Fatal("history[0] is not an object")
	}

	older, ok := history[1].(map[string]any)
	if !ok {
		t.Fatal("history[1] is not an object")
	}

	newestVer, _ := newest["version"].(float64)
	olderVer, _ := older["version"].(float64)
	if newestVer <= olderVer {
		t.Fatalf("Flux orders history newest-first: history[0].version=%v history[1].version=%v", newestVer, olderVer)
	}

	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "chartVersion", Path: "$.status.lastAttemptedRevision", Type: "string", Optional: true},
		{Name: "appVersion", Path: "$.status.history[0].appVersion", Type: "string", Optional: true},
		{Name: "revision", Path: "$.status.history[0].version", Type: "int", Optional: true},
		{Name: "valuesChecksum", Path: "$.status.lastAttemptedConfigDigest", Type: "string", Optional: true},
	}

	ext, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got, err := ext.Extract(&obj, attrs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got["chartVersion"] != "14.3.0" {
		t.Fatalf("chartVersion = %v, want 14.3.0 from lastAttemptedRevision", got["chartVersion"])
	}

	if got["appVersion"] != "2.1.0" {
		t.Fatalf("appVersion = %v, want 2.1.0 from history[0]", got["appVersion"])
	}

	if got["revision"] != int64(3) && got["revision"] != float64(3) && got["revision"] != 3 {
		t.Fatalf("revision = %v (%T), want 3 from history[0]", got["revision"], got["revision"])
	}

	if got["valuesChecksum"] != "sha256:abc123" {
		t.Fatalf("valuesChecksum = %v, want sha256:abc123", got["valuesChecksum"])
	}
}
