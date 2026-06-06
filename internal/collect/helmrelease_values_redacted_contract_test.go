// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestHelmReleaseValuesRedactedProfile_contract(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("testdata", "flux_helmrelease_values_sensitive.json")
	//nolint:gosec // G304: fixture path is fixed under package testdata
	raw, readErr := os.ReadFile(fixture)
	if readErr != nil {
		t.Fatalf("read fixture: %v", readErr)
	}

	var obj unstructured.Unstructured
	if unmarshalErr := json.Unmarshal(raw, &obj.Object); unmarshalErr != nil {
		t.Fatalf("decode fixture: %v", unmarshalErr)
	}

	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "releaseName", Path: "$.spec.releaseName", Type: "string", Optional: true},
		{Name: "chartVersion", Path: "$.status.lastAttemptedRevision", Type: "string", Optional: true},
		{Name: "appVersion", Path: "$.status.history[0].appVersion", Type: "string", Optional: true},
		{Name: "valuesChecksum", Path: "$.status.lastAttemptedConfigDigest", Type: "string", Optional: true},
		{Name: "values", Path: "$.spec.values", Optional: true},
	}

	ext, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got, err := ext.Extract(&obj, attrs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got = NewScrubber(nil).ScrubAttributes(got)

	if got["chartVersion"] != "14.3.0" {
		t.Fatalf("chartVersion = %v, want 14.3.0", got["chartVersion"])
	}

	if got["appVersion"] != "2.1.0" {
		t.Fatalf("appVersion = %v, want 2.1.0", got["appVersion"])
	}

	values, ok := got["values"].(map[string]any)
	if !ok {
		t.Fatalf("values type = %T, want map[string]any", got["values"])
	}

	wantValues := map[string]any{
		"image": map[string]any{
			"tag": "1.2.3",
		},
		"database": map[string]any{
			"host":     "postgres.internal",
			"password": redactedValue(),
		},
		"auth": redactedValue(),
	}

	if !reflect.DeepEqual(values, wantValues) {
		t.Fatalf("values = %#v, want %#v", values, wantValues)
	}
}
