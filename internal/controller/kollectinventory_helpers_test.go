// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

func TestExportErrorReason(t *testing.T) {
	t.Parallel()

	if got := sink.ExportErrorReason(nil); got != "unknown" {
		t.Fatalf("nil = %q", got)
	}
	if got := sink.ExportErrorReason(kollecterrors.Terminal(fmt.Errorf("bad"))); got != "terminal" {
		t.Fatalf("terminal = %q", got)
	}
}

func TestPerSinkCoalesceTracker_recordAndNextDue(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "team-a/inv"
	sinkName := "git"
	now := time.Now().UTC().Truncate(time.Second)
	tracker.record(invKey, sinkName, 1, "abc123fingerprint", now)

	if tracker.nextDue(invKey, sinkName, time.Minute, now) <= 0 {
		t.Fatal("expected positive nextDue after record")
	}
}

func TestKollectInventoryReconciler_maxExportBytes(t *testing.T) {
	t.Parallel()

	limit := int64(1024)
	rec := &KollectInventoryReconciler{}
	inv := &kollectdevv1alpha1.KollectInventory{
		Spec: kollectdevv1alpha1.KollectInventorySpec{MaxExportBytes: &limit},
	}
	if got := rec.maxExportBytes(inv); got != limit {
		t.Fatalf("maxExportBytes = %d", got)
	}
}
