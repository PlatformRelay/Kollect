// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/konih/kollect/internal/collect"
)

type fakeCopyTx struct {
	execErrors []error
	execCalls  int
	copyCalls  int
	copyRows   int
	copyErr    error
}

func (f *fakeCopyTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	idx := f.execCalls
	f.execCalls++
	if idx < len(f.execErrors) && f.execErrors[idx] != nil {
		return pgconn.CommandTag{}, f.execErrors[idx]
	}

	return pgconn.NewCommandTag("OK"), nil
}

func (f *fakeCopyTx) CopyFrom(
	_ context.Context,
	_ pgx.Identifier,
	_ []string,
	rowSrc pgx.CopyFromSource,
) (int64, error) {
	f.copyCalls++
	f.copyRows = 0
	for rowSrc.Next() {
		if _, err := rowSrc.Values(); err != nil {
			return 0, err
		}
		f.copyRows++
	}
	if err := rowSrc.Err(); err != nil {
		return 0, err
	}
	if f.copyErr != nil {
		return 0, f.copyErr
	}

	return int64(f.copyRows), nil
}

func TestUpsertItems_UsesRowPathBelowThreshold(t *testing.T) {
	t.Parallel()

	items := makeItems(bulkUpsertThreshold - 1)
	tx := &fakeCopyTx{}
	b := &Backend{}

	err := b.upsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items, time.Now().UTC())
	if err != nil {
		t.Fatalf("upsertItems() error = %v", err)
	}
	if tx.copyCalls != 0 {
		t.Fatalf("copyCalls = %d, want 0", tx.copyCalls)
	}
	if tx.execCalls != len(items) {
		t.Fatalf("execCalls = %d, want %d", tx.execCalls, len(items))
	}
}

func TestUpsertItems_UsesBulkPathAtThreshold(t *testing.T) {
	t.Parallel()

	items := makeItems(bulkUpsertThreshold)
	tx := &fakeCopyTx{}
	b := &Backend{}

	err := b.upsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items, time.Now().UTC())
	if err != nil {
		t.Fatalf("upsertItems() error = %v", err)
	}
	if tx.copyCalls != 1 {
		t.Fatalf("copyCalls = %d, want 1", tx.copyCalls)
	}
	if tx.copyRows != len(items) {
		t.Fatalf("copyRows = %d, want %d", tx.copyRows, len(items))
	}
	if tx.execCalls != 2 {
		t.Fatalf("execCalls = %d, want 2 (create + merge)", tx.execCalls)
	}
}

func TestRowUpsertItems_WrapsExecError(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{errors.New("db down")}}
	b := &Backend{}

	err := b.rowUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(1), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrUpsertFailed) {
		t.Fatalf("rowUpsertItems() error = %v, want ErrUpsertFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesCreateError(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{errors.New("create failed")}}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertCreateStagingFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want ErrBulkUpsertCreateStagingFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesCopyError(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{copyErr: errors.New("copy failed")}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertCopyFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want ErrBulkUpsertCopyFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesMergeError(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{nil, errors.New("merge failed")}}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertMergeFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want ErrBulkUpsertMergeFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesBuildRowError(t *testing.T) {
	t.Parallel()

	bad := []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
		Namespace:  "team-a",
		Attributes: map[string]any{"bad": math.NaN()},
	}}
	tx := &fakeCopyTx{}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", bad, time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want ErrBulkUpsertFailed", err)
	}
}

func makeItems(n int) []collect.Item {
	items := make([]collect.Item, 0, n)
	for i := range n {
		items = append(items, collect.Item{
			TargetName: "deployments",
			UID:        fmt.Sprintf("uid-%d", i),
			Namespace:  "team-a",
			Name:       "workload",
			Attributes: map[string]any{"replicas": i},
		})
	}

	return items
}
