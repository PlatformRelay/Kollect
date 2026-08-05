// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

func TestBqTableHandle_metadataAndCreateFailWithoutService(t *testing.T) {
	t.Parallel()

	client, err := bigquery.NewClient(
		t.Context(),
		"demo-project",
		option.WithoutAuthentication(),
		option.WithEndpoint("http://127.0.0.1:9"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	handle := bqTableHandle{table: client.Dataset("inventory").Table("items")}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if _, err := handle.Metadata(ctx); err == nil {
		t.Fatal("Metadata() error = nil, want request failure")
	}

	if err := handle.Create(ctx, &bigquery.TableMetadata{Name: "items"}); err == nil {
		t.Fatal("Create() error = nil, want request failure")
	}
}
