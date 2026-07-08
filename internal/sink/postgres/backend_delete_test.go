// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/konih/kollect/internal/collect"
)

type fakeDeleteTx struct {
	pgx.Tx
	execErrors []error
	execCalls  int
}

func (f *fakeDeleteTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	idx := f.execCalls
	f.execCalls++
	if idx < len(f.execErrors) && f.execErrors[idx] != nil {
		return pgconn.CommandTag{}, f.execErrors[idx]
	}

	return pgconn.NewCommandTag("OK"), nil
}

func TestDeleteStaleRows_DeleteAllPath(t *testing.T) {
	t.Parallel()

	tx := &fakeDeleteTx{}
	if err := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", nil); err != nil {
		t.Fatalf("deleteStaleRows() error = %v", err)
	}
	if tx.execCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", tx.execCalls)
	}
}

func TestDeleteStaleRows_DeleteStalePath(t *testing.T) {
	t.Parallel()

	tx := &fakeDeleteTx{}
	items := []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
	}}
	if err := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items); err != nil {
		t.Fatalf("deleteStaleRows() error = %v", err)
	}
	if tx.execCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", tx.execCalls)
	}
}

func TestDeleteStaleRows_WrapsExecError(t *testing.T) {
	t.Parallel()

	tx := &fakeDeleteTx{execErrors: []error{errors.New("db down")}}
	items := []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
	}}

	err := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items)
	if err == nil || !errors.Is(err, ErrDeleteStaleFailed) {
		t.Fatalf("deleteStaleRows() error = %v, want ErrDeleteStaleFailed", err)
	}
}
