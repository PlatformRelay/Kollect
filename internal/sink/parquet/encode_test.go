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
