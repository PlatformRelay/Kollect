// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestExport_DecodeErrorIsWrapped(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	err := b.Export(t.Context(), []byte(`{"schemaVersion":"kollect.dev/v99","items":[]}`), "")
	if err == nil || !errors.Is(err, ErrDecodePayloadFailed) {
		t.Fatalf("Export() error = %v, want wrapped ErrDecodePayloadFailed", err)
	}
}

func TestRowUpsertItems_WrapsExecErrorWithSentinel(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{errors.New("db down")}}
	b := &Backend{}

	err := b.rowUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(1), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrUpsertFailed) {
		t.Fatalf("rowUpsertItems() error = %v, want wrapped ErrUpsertFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesCreateErrorWithSentinel(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{errors.New("create failed")}}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertCreateStagingFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertCreateStagingFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesCopyErrorWithSentinel(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{copyErr: errors.New("copy failed")}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertCopyFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertCopyFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesMergeErrorWithSentinel(t *testing.T) {
	t.Parallel()

	tx := &fakeCopyTx{execErrors: []error{nil, errors.New("merge failed")}}
	b := &Backend{}

	err := b.bulkUpsertItems(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", makeItems(2), time.Now().UTC())
	if err == nil || !errors.Is(err, ErrBulkUpsertMergeFailed) {
		t.Fatalf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertMergeFailed", err)
	}
}

func TestBulkUpsertItems_PropagatesBuildRowErrorWithSentinel(t *testing.T) {
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
		t.Fatalf("bulkUpsertItems() error = %v, want wrapped ErrBulkUpsertFailed", err)
	}
}

func TestDeleteStaleRows_WrapsExecErrorWithSentinel(t *testing.T) {
	t.Parallel()

	tx := &fakeDeleteTx{execErrors: []error{errors.New("db down")}}
	items := []collect.Item{{
		TargetName: "deployments",
		UID:        "uid-1",
	}}

	err := deleteStaleRows(t.Context(), tx, `"public"."items"`, "team-a", "inventory", "cluster-a", items)
	if err == nil || !errors.Is(err, ErrDeleteStaleFailed) {
		t.Fatalf("deleteStaleRows() error = %v, want wrapped ErrDeleteStaleFailed", err)
	}
}
