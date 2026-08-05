// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

func TestClientQueryExecutor_executeFailsWithoutLiveService(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	exec := clientQueryExecutor{client: client}
	err = exec.Execute(ctx, "SELECT 1", nil, "EU")
	if err == nil {
		t.Fatal("Execute() error = nil, want run failure")
	}
	if !strings.Contains(err.Error(), "run:") {
		t.Fatalf("Execute() error = %v, want run: prefix", err)
	}
}

func TestBackend_executeQuery_usesInjectedExecutor(t *testing.T) {
	t.Parallel()

	exec := &fakeQueryExecutor{}
	b := &Backend{
		cfg:      Config{Location: "US"},
		executor: exec,
	}

	if err := b.executeQuery(t.Context(), "SELECT 1", nil); err != nil {
		t.Fatalf("executeQuery() error = %v", err)
	}
	if len(exec.calls) != 1 || exec.calls[0].location != "US" {
		t.Fatalf("calls = %#v, want one call with US location", exec.calls)
	}
}
