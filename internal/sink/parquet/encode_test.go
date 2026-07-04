// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package parquet

import (
	"bytes"
	"testing"
	"time"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/konih/kollect/internal/collect"
)

func TestEncodeItemsHybridSchema(t *testing.T) {
	t.Parallel()

	data, err := EncodeItems([]collect.Item{{
		TargetNamespace: "team-a",
		TargetName:      "deployments",
		Namespace:       "team-a",
		Name:            "api",
		Group:           "apps",
		Version:         "v1",
		Kind:            "Deployment",
		UID:             "uid-1",
		Attributes: map[string]any{
			"image":   "nginx:1.27",
			"version": "v3",
		},
	}}, EncodeOptions{
		Cluster:            "prod-west",
		InventoryNamespace: "team-a",
		InventoryName:      "deployments",
		HotAttributes:      []string{"image", "version"},
		ExportedAt:         time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	reader := parquetgo.NewReader(bytes.NewReader(data))
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Fatalf("reader.Close: %v", err)
		}
	})

	if reader.NumRows() != 1 {
		t.Fatalf("rows = %d, want 1", reader.NumRows())
	}

	type row struct {
		Cluster string `parquet:"cluster"`
		Name    string `parquet:"name"`
		Image   string `parquet:"attr_image,optional"`
		Version string `parquet:"attr_version,optional"`
	}

	var got row
	if err := reader.Read(&got); err != nil {
		t.Fatal(err)
	}

	if got.Cluster != "prod-west" || got.Name != "api" {
		t.Fatalf("identity = %+v", got)
	}

	if got.Image != "nginx:1.27" || got.Version != "v3" {
		t.Fatalf("promoted = (%q, %q)", got.Image, got.Version)
	}
}

func TestAttributeValue_branches(t *testing.T) {
	t.Parallel()

	// nil/empty map → nil (line 170-172)
	if attributeValue(nil, "k") != nil {
		t.Fatal("nil map must return nil")
	}
	if attributeValue(map[string]any{}, "k") != nil {
		t.Fatal("empty map must return nil")
	}

	// missing key → nil (line 174-176)
	if attributeValue(map[string]any{"x": "y"}, "missing") != nil {
		t.Fatal("missing key must return nil")
	}

	// nil value → nil
	if attributeValue(map[string]any{"k": nil}, "k") != nil {
		t.Fatal("nil value must return nil")
	}

	// string: empty → nil (line 181-183)
	if attributeValue(map[string]any{"k": ""}, "k") != nil {
		t.Fatal("empty string must return nil")
	}

	// string: non-empty → value (line 185-187)
	got := attributeValue(map[string]any{"k": "hello"}, "k")
	if got == nil || *got != "hello" {
		t.Fatalf("string value = %v, want hello", got)
	}

	// bool → formatted string (line 188-191)
	boolGot := attributeValue(map[string]any{"k": true}, "k")
	if boolGot == nil || *boolGot != "true" {
		t.Fatalf("bool value = %v, want true", boolGot)
	}

	// float64 → formatted string (line 192-195)
	f64Got := attributeValue(map[string]any{"k": float64(3.14)}, "k")
	if f64Got == nil || *f64Got == "" {
		t.Fatalf("float64 value = %v, want non-empty", f64Got)
	}

	// default: marshallable struct → JSON string (line 196-204)
	mapGot := attributeValue(map[string]any{"k": map[string]any{"nested": "val"}}, "k")
	if mapGot == nil || *mapGot == "" {
		t.Fatalf("map value = %v, want JSON", mapGot)
	}
}

func TestNormalizeHotAttributes_edgeCases(t *testing.T) {
	t.Parallel()

	// empty list → DefaultHotAttributes
	got := normalizeHotAttributes(nil)
	if len(got) == 0 {
		t.Fatal("nil attrs must default to DefaultHotAttributes")
	}

	// empty strings filtered out (line 100-102)
	got = normalizeHotAttributes([]string{"a", "", "b"})
	for _, attr := range got {
		if attr == "" {
			t.Fatal("empty string must be filtered out")
		}
	}

	// duplicates deduplicated (line 104-107)
	got = normalizeHotAttributes([]string{"image", "IMAGE", "version"})
	seen := map[string]int{}
	for _, attr := range got {
		seen[attr]++
	}
	for attr, count := range seen {
		if count > 1 {
			t.Fatalf("duplicate attr %q in result", attr)
		}
	}
}

func TestSanitizeColumn_edgeCases(t *testing.T) {
	t.Parallel()

	// empty string → "unknown" (line 148-150)
	if got := sanitizeColumn(""); got != "unknown" {
		t.Fatalf("empty → %q, want unknown", got)
	}

	// whitespace-only → "unknown"
	if got := sanitizeColumn("   "); got != "unknown" {
		t.Fatalf("whitespace → %q, want unknown", got)
	}

	// all invalid chars → b.Len()==0 → "unknown" (line 162-164)
	if got := sanitizeColumn("$$$"); got != "unknown" {
		t.Fatalf("all-invalid → %q, want unknown", got)
	}

	// mixed valid/invalid chars
	if got := sanitizeColumn("app$version"); got != "appversion" {
		t.Fatalf("mixed → %q, want appversion", got)
	}
}

func TestEncodeItemsEmptySnapshot(t *testing.T) {
	t.Parallel()

	data, err := EncodeItems(nil, EncodeOptions{
		InventoryNamespace: "team-a",
		InventoryName:      "deployments",
	})
	if err != nil {
		t.Fatal(err)
	}

	reader := parquetgo.NewReader(bytes.NewReader(data))
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Fatalf("reader.Close: %v", err)
		}
	})

	if reader.NumRows() != 0 {
		t.Fatalf("rows = %d, want 0", reader.NumRows())
	}
}
