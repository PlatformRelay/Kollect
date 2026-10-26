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

// TestAggregateInventorySync pins the aggregate rule of SDC-2 and SDC-3: no failed sink means
// Synced=True/Exported with one message for export and debounce passes; any failure keeps False.
func TestAggregateInventorySync(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                        string
		exported, debounced, failed int
		wantStatus                  metav1.ConditionStatus
		wantReason                  string
		wantMessage                 string
	}{
		{"all exported", 2, 0, 0, metav1.ConditionTrue, reasonExported, "2/2 sink(s) up to date"},
		{"all debounced", 0, 2, 0, metav1.ConditionTrue, reasonExported, "2/2 sink(s) up to date"},
		{"one exported, one debounced", 1, 1, 0, metav1.ConditionTrue, reasonExported, "2/2 sink(s) up to date"},
		{"one exported, one failed", 1, 0, 1, metav1.ConditionFalse, kollectdevv1alpha1.ReasonPartiallySynced, ""},
		{"one debounced, one failed", 0, 1, 1, metav1.ConditionFalse, reasonProgressing, ""},
		{"only failed", 0, 0, 1, metav1.ConditionFalse, reasonProgressing, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var conditions []metav1.Condition
			aggregateInventorySync(&conditions, 1, tc.exported, tc.debounced, tc.failed)
			synced := apimeta.FindStatusCondition(conditions, conditionSynced)
			if synced == nil {
				t.Fatal("aggregate Synced condition not set")
			}
			if synced.Status != tc.wantStatus || synced.Reason != tc.wantReason {
				t.Fatalf("aggregate Synced = %s/%s %q, want %s/%s",
					synced.Status, synced.Reason, synced.Message, tc.wantStatus, tc.wantReason)
			}
			if tc.wantMessage != "" && synced.Message != tc.wantMessage {
				t.Fatalf("aggregate Synced message = %q, want %q", synced.Message, tc.wantMessage)
			}
		})
	}
}

// TestSetSinkExportDebounced pins SDC-1: a debounced sink reads Synced=True/Debounced and its
// message names the interval with nothing that changes between reconciles.
func TestSetSinkExportDebounced(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		interval time.Duration
		want     string
	}{
		{"interval 15m", 15 * time.Minute, "15m0s"},
		{"interval 0s", 0, "only on change"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "database/pg"}
			setSinkExportDebounced(status, 3, tc.interval)
			first := apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
			if first == nil {
				t.Fatal("per-sink Synced condition not set")
			}
			firstCopy := *first
			if firstCopy.Status != metav1.ConditionTrue || firstCopy.Reason != kollectdevv1alpha1.ReasonDebounced {
				t.Fatalf("per-sink Synced = %s/%s, want True/Debounced", firstCopy.Status, firstCopy.Reason)
			}
			if !strings.Contains(firstCopy.Message, tc.want) {
				t.Fatalf("per-sink Synced message = %q, want it to contain %q", firstCopy.Message, tc.want)
			}
			if firstCopy.ObservedGeneration != 3 {
				t.Fatalf("observedGeneration = %d, want 3", firstCopy.ObservedGeneration)
			}

			time.Sleep(1100 * time.Millisecond)
			setSinkExportDebounced(status, 3, tc.interval)
			second := apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
			if second.Message != firstCopy.Message {
				t.Fatalf("per-sink Synced message changed between reconciles: %q -> %q", firstCopy.Message, second.Message)
			}
			if !second.LastTransitionTime.Equal(&firstCopy.LastTransitionTime) {
				t.Fatalf("per-sink lastTransitionTime moved: %v -> %v", firstCopy.LastTransitionTime, second.LastTransitionTime)
			}
		})
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

	if tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", 0, now) {
		t.Fatal("first export must not skip")
	}

	tracker.record(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", now)
	if !tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", 0, now) {
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

	if tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-1", interval, now) {
		t.Fatal("first export must not skip, even with a requestedAt set")
	}

	tracker.record(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-1", now)
	if !tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-1", interval, now) {
		t.Fatal("unchanged requestedAt within interval should skip")
	}

	if tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-2", interval, now) {
		t.Fatal("a different requestedAt value must bypass the debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", interval, now) {
		t.Fatal("removal of the annotation must bypass the debounce (absence is a value)")
	}

	tracker.record(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", now)
	if !tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "", interval, now) {
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

	tracker.record(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-1", now)
	if !tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-1", 0, now) {
		t.Fatal("zero-interval binding should skip an identical payload (material-change-only cadence)")
	}

	if tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-2", 0, now) {
		t.Fatal("zero-interval binding must still re-export on a requestedAt change")
	}

	tracker.record(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-2", now)
	if !tracker.shouldSkip(invKey, sinkName, sinkExportIdentity{}, 1, "hash", "ts-2", 0, now) {
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
	tracker.record(staleInvKey, staleSink, sinkExportIdentity{}, 1, "hash-v1", "", base)

	// Active entry: keeps recording well within the TTL window.
	activeNow := base
	for i := 0; i < 3; i++ {
		activeNow = activeNow.Add(coalesceStateTTL / 4)
		tracker.record(activeInvKey, activeSink, sinkExportIdentity{}, int64(i), "hash-active", "", activeNow)
	}

	// Long after the stale entry's TTL has elapsed, the active sink records
	// again. This is the prune trigger point: any record() call sweeps the
	// whole map for entries that have aged out.
	farFuture := base.Add(coalesceStateTTL + time.Minute)
	tracker.record(activeInvKey, activeSink, sinkExportIdentity{}, 5, "hash-final", "", farFuture)

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
	if !tracker.shouldSkip(activeInvKey, activeSink, sinkExportIdentity{}, 5, "hash-final", "", 0, farFuture) {
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

// carryOverSinkExport seeds a rebuilt entry from the previous entry of the same name: the last
// export time, checksum, recorded paths and the conditions, deep-copied (SDC-5).
func TestCarryOverSinkExport(t *testing.T) {
	t.Parallel()

	exportedAt := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	t1 := metav1.NewTime(exportedAt)
	l1 := metav1.NewTime(time.Date(2026, 9, 1, 7, 0, 0, 0, time.UTC))
	previous := []kollectdevv1alpha1.InventorySinkExportStatus{
		{Name: "database/other", LastChecksum: "other"},
		{
			Name:            "snapshot/git",
			LastExportTime:  &t1,
			LastChecksum:    "c1",
			LastExportPaths: []string{"inventory/team-a/inv.yaml", "inventory/team-a/inv-2.yaml"},
			Conditions: []metav1.Condition{{
				Type: conditionSinkSynced, Status: metav1.ConditionTrue, Reason: reasonExported,
				Message: "export completed", LastTransitionTime: l1,
			}},
		},
	}

	entry := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "snapshot/git"}
	carryOverSinkExport(entry, previous, "snapshot/git")

	if entry.LastExportTime == nil || !entry.LastExportTime.Equal(&t1) {
		t.Fatalf("lastExportTime = %v, want %v carried over", entry.LastExportTime, t1)
	}
	if entry.LastChecksum != "c1" {
		t.Fatalf("lastChecksum = %q, want %q carried over", entry.LastChecksum, "c1")
	}
	if len(entry.LastExportPaths) != 2 || entry.LastExportPaths[0] != "inventory/team-a/inv.yaml" {
		t.Fatalf("lastExportPaths = %v, want both recorded paths carried over", entry.LastExportPaths)
	}
	synced := apimeta.FindStatusCondition(entry.Conditions, conditionSinkSynced)
	if synced == nil {
		t.Fatalf("conditions = %v, want the previous Synced condition carried over", entry.Conditions)
	}
	if synced.Status != metav1.ConditionTrue || synced.Reason != reasonExported || !synced.LastTransitionTime.Equal(&l1) {
		t.Fatalf("carried Synced = %s/%s at %v, want True/%s at %v",
			synced.Status, synced.Reason, synced.LastTransitionTime, reasonExported, l1)
	}

	// The seed is a deep copy: changing the new entry leaves the previous one unchanged.
	setSinkExportSynced(entry, 2, false, reasonExportFailed, "sink unavailable")
	entry.LastExportPaths[0] = "changed.yaml"
	*entry.LastExportTime = metav1.NewTime(t1.Add(time.Hour))
	prevSynced := apimeta.FindStatusCondition(previous[1].Conditions, conditionSinkSynced)
	if prevSynced.Status != metav1.ConditionTrue || prevSynced.Reason != reasonExported || !prevSynced.LastTransitionTime.Equal(&l1) {
		t.Fatalf("previous Synced = %s/%s at %v after changing the new entry, want True/%s at %v unchanged",
			prevSynced.Status, prevSynced.Reason, prevSynced.LastTransitionTime, reasonExported, l1)
	}
	if previous[1].LastExportPaths[0] != "inventory/team-a/inv.yaml" {
		t.Fatalf("previous paths = %v after changing the new entry, want them unchanged", previous[1].LastExportPaths)
	}
	if !previous[1].LastExportTime.Time.Equal(exportedAt) {
		t.Fatalf("previous lastExportTime = %v after changing the new entry, want %v", previous[1].LastExportTime, exportedAt)
	}

	// No previous entry of that name: the entry stays empty.
	fresh := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "database/pg"}
	carryOverSinkExport(fresh, previous, "database/pg")
	if fresh.LastExportTime != nil || fresh.LastChecksum != "" || len(fresh.LastExportPaths) != 0 || len(fresh.Conditions) != 0 {
		t.Fatalf("entry with no previous of its name = %+v, want empty", fresh)
	}

	// Nil entry or no previous entries: no-op, not a panic.
	carryOverSinkExport(nil, previous, "snapshot/git")
	carryOverSinkExport(fresh, nil, "database/pg")
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
