// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/validation"
)

func TestMergeRequeueAfter(t *testing.T) {
	t.Parallel()

	if got := mergeRequeueAfter(0, time.Minute); got != time.Minute {
		t.Fatalf("zero current = %v, want 1m", got)
	}
	if got := mergeRequeueAfter(2*time.Minute, time.Minute); got != time.Minute {
		t.Fatalf("shorter next = %v, want 1m", got)
	}
	if got := mergeRequeueAfter(time.Minute, 2*time.Minute); got != time.Minute {
		t.Fatalf("keep earlier = %v, want 1m", got)
	}
}

func TestUpsertSinkExportStatus(t *testing.T) {
	t.Parallel()

	var exports []kollectdevv1alpha1.InventorySinkExportStatus
	first := upsertSinkExportStatus(&exports, "git")
	if first == nil || first.Name != "git" || len(exports) != 1 {
		t.Fatalf("first upsert = %#v, exports = %#v", first, exports)
	}

	second := upsertSinkExportStatus(&exports, "git")
	if second != first || len(exports) != 1 {
		t.Fatal("duplicate name should return existing entry")
	}

	third := upsertSinkExportStatus(&exports, "s3")
	if third == nil || third.Name != "s3" || len(exports) != 2 {
		t.Fatalf("second sink = %#v, exports = %#v", third, exports)
	}
}

func TestSetSinkExportSynced_nilStatusIsNoop(t *testing.T) {
	t.Parallel()

	// nil status must not panic
	setSinkExportSynced(nil, 1, true, "Exported", "ok")
}

func TestSetSinkExportSynced(t *testing.T) {
	t.Parallel()

	status := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "git"}
	setSinkExportSynced(status, 3, true, "Exported", "ok")

	synced := apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
	if synced == nil {
		t.Fatal("Synced condition missing")
	}
	if synced.Status != metav1.ConditionTrue || synced.Reason != "Exported" {
		t.Fatalf("condition = %#v", synced)
	}
	if synced.ObservedGeneration != 3 {
		t.Fatalf("generation = %d, want 3", synced.ObservedGeneration)
	}

	setSinkExportSynced(status, 4, false, "ExportFailed", "boom")
	synced = apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
	if synced.Status != metav1.ConditionFalse || synced.Reason != "ExportFailed" {
		t.Fatalf("failed condition = %#v", synced)
	}
}

func TestAggregateInventorySync(t *testing.T) {
	t.Parallel()

	var conditions []metav1.Condition
	aggregateInventorySync(&conditions, 1, 2, 0, 0)
	synced := apimeta.FindStatusCondition(conditions, conditionSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue || synced.Reason != "Exported" {
		t.Fatalf("exported = %#v", synced)
	}

	conditions = nil
	aggregateInventorySync(&conditions, 1, 0, 2, 0)
	synced = apimeta.FindStatusCondition(conditions, conditionSynced)
	if synced == nil || synced.Reason != kollectdevv1alpha1.ReasonPartiallySynced {
		t.Fatalf("all debounced = %#v", synced)
	}

	conditions = nil
	aggregateInventorySync(&conditions, 1, 1, 1, 0)
	synced = apimeta.FindStatusCondition(conditions, conditionSynced)
	if synced == nil || synced.Reason != kollectdevv1alpha1.ReasonPartiallySynced {
		t.Fatalf("partial debounce = %#v", synced)
	}

	conditions = nil
	aggregateInventorySync(&conditions, 1, 1, 0, 1)
	synced = apimeta.FindStatusCondition(conditions, conditionSynced)
	if synced == nil || synced.Reason != kollectdevv1alpha1.ReasonPartiallySynced {
		t.Fatalf("partial export failure = %#v", synced)
	}

	conditions = nil
	aggregateInventorySync(&conditions, 1, 0, 0, 1)
	synced = apimeta.FindStatusCondition(conditions, conditionSynced)
	if synced == nil || synced.Reason != "Progressing" {
		t.Fatalf("failed = %#v", synced)
	}
}

func TestLatestExportTime(t *testing.T) {
	t.Parallel()

	early := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	late := metav1.NewTime(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	exports := []kollectdevv1alpha1.InventorySinkExportStatus{
		{Name: "a", LastExportTime: &early},
		{Name: "b", LastExportTime: &late},
		{Name: "c"},
	}

	got := latestExportTime(exports)
	if got == nil || !got.Equal(&late) {
		t.Fatalf("latest = %v, want %v", got, late)
	}
	if latestExportTime(nil) != nil {
		t.Fatal("nil slice should return nil")
	}
}

func TestPerSinkCoalesceTracker_nextDue_zeroInterval(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	got := tracker.nextDue("default/inv", "git", 0, time.Now())
	if got != validation.ZeroIntervalWatchdog {
		t.Fatalf("zero interval nextDue = %v, want watchdog", got)
	}
}

func TestPerSinkCoalesceTracker_shouldSkip_zeroIntervalAfterRecord(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	now := time.Now()
	invKey := "default/inv"
	sinkName := "git"

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", 0, now) {
		t.Fatal("first export must not skip")
	}

	tracker.record(invKey, sinkName, 1, "hash", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", 0, now) {
		t.Fatal("material-change-only cadence should skip identical payload")
	}
}

// EC-P1-06: per-sink failures must aggregate instead of last-write-wins.
func TestPerSinkExportOutcome_addSinkFailure_aggregates(t *testing.T) {
	t.Parallel()

	var outcome perSinkExportOutcome
	outcome.addSinkFailure("snapshot/git-main", kollecterrors.Terminal(errors.New("repo not found")))
	outcome.addSinkFailure("database/pg-demo", kollecterrors.Transient(errors.New("connection refused")))

	if outcome.FailedCount != 2 {
		t.Fatalf("FailedCount = %d, want 2", outcome.FailedCount)
	}
	if outcome.FailedSink != "snapshot/git-main,database/pg-demo" {
		t.Fatalf("FailedSink = %q, want both failed sink keys", outcome.FailedSink)
	}

	msg := outcome.ExportErr.Error()
	if !strings.Contains(msg, "repo not found") || !strings.Contains(msg, "connection refused") {
		t.Fatalf("ExportErr = %q, want all component failure messages", msg)
	}
}

func TestAggregateExportErrs_allTerminalIsTerminal(t *testing.T) {
	t.Parallel()

	err := aggregateExportErrs([]error{
		kollecterrors.Terminal(errors.New("bucket missing")),
		kollecterrors.Terminal(errors.New("bad credentials secret")),
	})
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("ClassOf = %q, want terminal when all components are terminal", kollecterrors.ClassOf(err))
	}
}

func TestAggregateExportErrs_mixedIsTransient(t *testing.T) {
	t.Parallel()

	// Terminal first: errors.As DFS order would pick the terminal class if the
	// aggregate were not explicitly re-classified.
	err := aggregateExportErrs([]error{
		kollecterrors.Terminal(errors.New("bucket missing")),
		kollecterrors.Transient(errors.New("connection refused")),
	})
	if kollecterrors.IsTerminal(err) {
		t.Fatal("mixed aggregate classified terminal, want transient so retry still happens")
	}
	if !kollecterrors.IsTransient(err) {
		t.Fatalf("ClassOf = %q, want transient", kollecterrors.ClassOf(err))
	}
}

func TestAggregateExportErrs_singleKeepsClass(t *testing.T) {
	t.Parallel()

	terminal := kollecterrors.Terminal(errors.New("schema invalid"))
	if got := aggregateExportErrs([]error{terminal}); !kollecterrors.IsTerminal(got) {
		t.Fatalf("single terminal aggregate ClassOf = %q, want terminal", kollecterrors.ClassOf(got))
	}

	if got := aggregateExportErrs(nil); got != nil {
		t.Fatalf("empty aggregate = %v, want nil", got)
	}
}
