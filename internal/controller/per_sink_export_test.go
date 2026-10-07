// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/validation"
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

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", "", 0, now) {
		t.Fatal("first export must not skip")
	}

	tracker.record(invKey, sinkName, 1, "hash", "", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "", 0, now) {
		t.Fatal("material-change-only cadence should skip identical payload")
	}
}

// ERA-1 third axis at the unit level: the kollect.dev/requestedAt value is a
// debounce axis of its own. Any change — absence→present, present→absence, or
// a different value — invalidates a within-interval state once; recording the
// new value resumes the steady-state debounce. The value is not parsed: only
// equality against the recorded value matters.
func TestPerSinkCoalesceTracker_requestedAtAxis(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "default/inv"
	sinkName := "git"
	interval := time.Minute
	now := time.Now()

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-1", interval, now) {
		t.Fatal("first export must not skip, even with a requestedAt set")
	}

	tracker.record(invKey, sinkName, 1, "hash", "ts-1", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-1", interval, now) {
		t.Fatal("unchanged requestedAt within interval should skip")
	}

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-2", interval, now) {
		t.Fatal("a different requestedAt value must bypass the debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", "", interval, now) {
		t.Fatal("removal of the annotation must bypass the debounce (absence is a value)")
	}

	tracker.record(invKey, sinkName, 1, "hash", "", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "", interval, now) {
		t.Fatal("steady absence should skip again after the forced export recorded it")
	}
}

// Pins the shouldSkip ordering the ERA-1 comment claims: the requestedAt
// comparison sits before the zero-interval early return, so a binding with
// exportMinInterval: 0 still re-exports on a requestedAt change instead of
// hitting the material-change-only skip. A future reorder breaks this test
// before it breaks production.
func TestPerSinkCoalesceTracker_requestedAt_zeroIntervalStillReexports(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "default/inv"
	sinkName := "git"
	now := time.Now()

	tracker.record(invKey, sinkName, 1, "hash", "ts-1", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-1", 0, now) {
		t.Fatal("zero-interval binding should skip an identical payload (material-change-only cadence)")
	}

	if tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-2", 0, now) {
		t.Fatal("zero-interval binding must still re-export on a requestedAt change")
	}

	tracker.record(invKey, sinkName, 1, "hash", "ts-2", now)
	if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "ts-2", 0, now) {
		t.Fatal("zero-interval binding resumes skipping after the forced export recorded the new value")
	}
}

// EC-P1-06: per-sink failures must aggregate instead of last-write-wins.
// AR-11: the tracker is a long-lived field on each reconciler and must not
// grow unbounded as inventories/sinks churn over the life of the controller
// process. Entries that haven't recorded an export in coalesceStateTTL are
// stale (their owning inventory/sink was almost certainly deleted) and must
// be pruned, while entries still actively exporting must survive untouched.
func TestPerSinkCoalesceTracker_prunesStaleEntries(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	base := time.Now()

	staleInvKey, staleSink := "default/inv-stale", "git"
	activeInvKey, activeSink := "default/inv-active", "git"

	// Stale entry: recorded once, then its owning inventory/sink is deleted
	// and nothing ever touches this key again.
	tracker.record(staleInvKey, staleSink, 1, "hash-v1", "", base)

	// Active entry: keeps recording well within the TTL window.
	activeNow := base
	for i := 0; i < 3; i++ {
		activeNow = activeNow.Add(coalesceStateTTL / 4)
		tracker.record(activeInvKey, activeSink, int64(i), "hash-active", "", activeNow)
	}

	// Long after the stale entry's TTL has elapsed, the active sink records
	// again. This is the prune trigger point: any record() call sweeps the
	// whole map for entries that have aged out.
	farFuture := base.Add(coalesceStateTTL + time.Minute)
	tracker.record(activeInvKey, activeSink, 5, "hash-final", "", farFuture)

	tracker.mu.Lock()
	_, staleStillPresent := tracker.states[tracker.key(staleInvKey, staleSink)]
	_, activeStillPresent := tracker.states[tracker.key(activeInvKey, activeSink)]
	tracker.mu.Unlock()

	if staleStillPresent {
		t.Fatal("stale entry should have been pruned after coalesceStateTTL elapsed with no activity")
	}
	if !activeStillPresent {
		t.Fatal("active entry must survive the sweep")
	}

	// The active entry must still coalesce correctly after surviving a sweep:
	// an identical payload at the same generation should be skipped.
	if !tracker.shouldSkip(activeInvKey, activeSink, 5, "hash-final", "", 0, farFuture) {
		t.Fatal("active entry's coalescing state must remain intact after a sweep")
	}
}

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

// recordExportPaths deduplicates, sorts, and caps the written paths; an empty
// write keeps the previous recording because an export that wrote nothing did
// not invalidate the paths recorded before it.
func TestRecordExportPaths(t *testing.T) {
	t.Parallel()

	previous := []string{"inventory/team-a/inv.yaml"}

	if got := recordExportPaths(nil, previous); len(got) != 1 || got[0] != previous[0] {
		t.Fatalf("nil write = %v, want the previous recording kept", got)
	}
	if got := recordExportPaths([]string{}, previous); len(got) != 1 || got[0] != previous[0] {
		t.Fatalf("empty write = %v, want the previous recording kept", got)
	}

	got := recordExportPaths([]string{"b.yaml", "a.yaml", "b.yaml", "c.yaml"}, previous)
	want := []string{"a.yaml", "b.yaml", "c.yaml"}
	if len(got) != len(want) {
		t.Fatalf("recordExportPaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recordExportPaths = %v, want sorted unique %v", got, want)
		}
	}

	flood := make([]string, 0, maxRecordedExportPaths+5)
	for i := 0; i < maxRecordedExportPaths+5; i++ {
		flood = append(flood, fmt.Sprintf("p/%03d.yaml", i))
	}
	if got := recordExportPaths(flood, nil); len(got) != maxRecordedExportPaths {
		t.Fatalf("capped len = %d, want %d", len(got), maxRecordedExportPaths)
	}
}

// carryOverLastExportPaths keeps the previous entry's recorded paths on a
// rebuilt status entry, so a debounced or failed reconcile never erases the
// cleanup evidence.
func TestCarryOverLastExportPaths(t *testing.T) {
	t.Parallel()

	recorded := []kollectdevv1alpha1.InventorySinkExportStatus{{
		Name:            "snapshot/git",
		LastExportPaths: []string{"inventory/team-a/inv.yaml"},
	}}

	rebuilt := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "snapshot/git"}
	carryOverLastExportPaths(rebuilt, recorded, "snapshot/git")
	if len(rebuilt.LastExportPaths) != 1 {
		t.Fatalf("rebuilt paths = %v, want the recorded paths carried over", rebuilt.LastExportPaths)
	}

	// A fresh success already wrote new paths: the carry-over must not win.
	fresh := &kollectdevv1alpha1.InventorySinkExportStatus{
		Name:            "snapshot/git",
		LastExportPaths: []string{"inventory/team-a/inv-v2.yaml"},
	}
	carryOverLastExportPaths(fresh, recorded, "snapshot/git")
	if len(fresh.LastExportPaths) != 1 || fresh.LastExportPaths[0] != "inventory/team-a/inv-v2.yaml" {
		t.Fatalf("fresh paths = %v, want the new export's paths kept", fresh.LastExportPaths)
	}

	// No previous entry and no export key match: no-op, not a panic.
	carryOverLastExportPaths(rebuilt, nil, "other")
	carryOverLastExportPaths(nil, recorded, "snapshot/git")
}

// exportEvidenceBySink indexes the export status by export key: the recorded
// paths, and whether the sink saw a past export at all (MR-01: an export with
// no recorded paths cannot vouch for a clean retraction).
func TestExportEvidenceBySink(t *testing.T) {
	t.Parallel()

	exported := metav1.Now()
	got := exportEvidenceBySink([]kollectdevv1alpha1.InventorySinkExportStatus{
		{Name: "snapshot/git", LastExportTime: &exported, LastExportPaths: []string{"inventory/team-a/inv.yaml"}},
		{Name: "snapshot/legacy", LastExportTime: &exported},
		{Name: "database/pg"},
	})

	if ev := got["snapshot/git"]; !ev.exported || len(ev.paths) != 1 || ev.paths[0] != "inventory/team-a/inv.yaml" {
		t.Fatalf("snapshot/git evidence = %+v, want exported with the recorded document path", ev)
	}
	if ev := got["snapshot/legacy"]; !ev.exported || len(ev.paths) != 0 {
		t.Fatalf("snapshot/legacy evidence = %+v, want exported without recorded paths", ev)
	}
	if ev := got["database/pg"]; ev.exported || len(ev.paths) != 0 {
		t.Fatalf("database/pg evidence = %+v, want never exported", ev)
	}
	if ev, ok := got["snapshot/unknown"]; ok || ev.exported {
		t.Fatalf("unknown sink evidence = %+v, want zero value", ev)
	}
}
