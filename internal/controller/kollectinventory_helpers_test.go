// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func TestKollectInventoryReconciler_lastExportTime(t *testing.T) {
	t.Parallel()

	rec := &KollectInventoryReconciler{}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 1},
	}
	key := "team-a/inv"
	hash := "abc123fingerprint"

	if !rec.lastExportTime(key).IsZero() {
		t.Fatal("expected zero time before recordExport")
	}

	rec.recordExport(inv, key, hash)
	if rec.lastExportTime(key).IsZero() {
		t.Fatal("expected recorded export time")
	}

	if time.Since(rec.lastExportTime(key)) > time.Minute {
		t.Fatal("export time too old")
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
