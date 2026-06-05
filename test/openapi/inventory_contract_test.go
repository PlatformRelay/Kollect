// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package openapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/inventory"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestInventorySummaryMatchesGoTypes(t *testing.T) {
	t.Parallel()

	sample := inventory.InventorySummary{
		SchemaVersion: collect.ExportSchemaVersion,
		ItemCount:     1,
		Namespace:     "team-a",
		Inventory:     "demo",
		UpdatedAt:     "2026-06-05T12:00:00Z",
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "deploys",
			Namespace:       "apps",
			Name:            "web",
			Group:           "apps",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 3},
		}},
		Pagination: &inventory.Pagination{Limit: 500, Offset: 0, Total: 1, HasMore: false},
		ExportStatus: []inventory.ExportStatus{{
			SinkName: "git",
			Status:   "ok",
		}},
	}

	encoded, err := json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}

	var decoded inventory.InventorySummary
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.SchemaVersion != collect.ExportSchemaVersion {
		t.Fatalf("schemaVersion = %q", decoded.SchemaVersion)
	}
	if len(decoded.Items) != 1 || decoded.Items[0].Kind != "Deployment" {
		t.Fatalf("items = %#v", decoded.Items)
	}
}

func TestOpenAPIInventorySpecPresent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repoRoot(t), "openapi", "v1alpha1", "inventory.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	body := string(data)
	for _, fragment := range []string{
		"InventorySummary",
		"schemaVersion",
		"/v1alpha1/status/targets",
		"/v1alpha1/status/inventories",
		"ExportStatus",
		"Pagination",
	} {
		if !contains(body, fragment) {
			t.Fatalf("openapi missing %q", fragment)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}

	return -1
}
