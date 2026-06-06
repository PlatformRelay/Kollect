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

func TestHelmReleaseSecretSummaryProfile_contract(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("testdata", "helm_release_secret.json")
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
		{Name: "releaseName", Path: "helm:release.releaseName", Type: "string", Optional: true},
		{Name: "chartVersion", Path: "helm:release.chartVersion", Type: "string", Optional: true},
		{Name: "appVersion", Path: "helm:release.appVersion", Type: "string", Optional: true},
		{Name: "revision", Path: "helm:release.revision", Type: "int", Optional: true},
		{Name: "status", Path: "helm:release.status", Type: "string", Optional: true},
		{Name: "lastDeployed", Path: "helm:release.lastDeployed", Type: "string", Optional: true},
	}

	ext, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got, err := ext.Extract(&obj, attrs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got["releaseName"] != "my-app" {
		t.Fatalf("releaseName = %v, want my-app", got["releaseName"])
	}

	if got["chartVersion"] != "14.3.0" {
		t.Fatalf("chartVersion = %v, want 14.3.0", got["chartVersion"])
	}

	if got["appVersion"] != "2.1.0" {
		t.Fatalf("appVersion = %v, want 2.1.0", got["appVersion"])
	}

	if got["revision"] != int64(2) && got["revision"] != float64(2) && got["revision"] != 2 {
		t.Fatalf("revision = %v (%T), want 2", got["revision"], got["revision"])
	}

	if got["status"] != "deployed" {
		t.Fatalf("status = %v, want deployed", got["status"])
	}

	if got["lastDeployed"] != "2024-01-15T10:30:00Z" {
		t.Fatalf("lastDeployed = %v, want 2024-01-15T10:30:00Z", got["lastDeployed"])
	}
}
