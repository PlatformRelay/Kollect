// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"fmt"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
)

// SDC-4: the debounce baseline holds the inventory object and the family sink object of the
// last successful export; a different object behind the same names re-exports.
func TestPerSinkCoalesceTracker_identity(t *testing.T) {
	t.Parallel()

	recorded := sinkExportIdentity{inventoryUID: "inv-uid-1", sinkUID: "sink-uid-1", sinkGeneration: 3}
	rows := []struct {
		name     string
		current  sinkExportIdentity
		wantSkip bool
	}{
		{"same identity", recorded, true},
		{"other inventory UID", sinkExportIdentity{inventoryUID: "inv-uid-2", sinkUID: "sink-uid-1", sinkGeneration: 3}, false},
		{"other sink UID", sinkExportIdentity{inventoryUID: "inv-uid-1", sinkUID: "sink-uid-2", sinkGeneration: 3}, false},
		{"other sink generation", sinkExportIdentity{inventoryUID: "inv-uid-1", sinkUID: "sink-uid-1", sinkGeneration: 4}, false},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			var tracker perSinkCoalesceTracker
			now := time.Now()
			tracker.record("default/inv", "database/pg", recorded, 1, "hash", "", now)
			got := tracker.shouldSkip("default/inv", "database/pg", row.current, 1, "hash", "", 15*time.Minute, now)
			if got != row.wantSkip {
				t.Fatalf("shouldSkip = %v, want %v (recorded %+v, current %+v)", got, row.wantSkip, recorded, row.current)
			}
		})
	}
}

func TestNewSinkExportIdentity(t *testing.T) {
	t.Parallel()

	got := newSinkExportIdentity("inv-uid", &sink.ResolvedSink{UID: "sink-uid", Generation: 7})
	want := sinkExportIdentity{inventoryUID: "inv-uid", sinkUID: "sink-uid", sinkGeneration: 7}
	if got != want {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
	if got := newSinkExportIdentity("inv-uid", nil); got != (sinkExportIdentity{inventoryUID: "inv-uid"}) {
		t.Fatalf("identity without a sink = %+v, want the inventory UID only", got)
	}
}

// SDC-4: a failed export clears the debounce state, so the sink is not debounced until an
// export to it succeeds again.
func TestPerSinkCoalesceTracker_forget(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	identity := sinkExportIdentity{inventoryUID: "inv-uid", sinkUID: "sink-uid", sinkGeneration: 1}
	now := time.Now()

	tracker.forget("default/inv", "database/pg") // zero-value tracker: no-op, no panic

	tracker.record("default/inv", "database/pg", identity, 1, "c1", "", now)
	tracker.record("default/inv", "database/other", identity, 1, "c1", "", now)

	tracker.forget("default/inv", "database/unknown")
	if !tracker.shouldSkip("default/inv", "database/pg", identity, 1, "c1", "", 15*time.Minute, now) {
		t.Fatal("forget of an unknown key must leave other entries untouched")
	}

	tracker.forget("default/inv", "database/pg")
	if tracker.shouldSkip("default/inv", "database/pg", identity, 1, "c1", "", 15*time.Minute, now) {
		t.Fatal("shouldSkip after forget = true, want false: the sink must export again")
	}
	if !tracker.shouldSkip("default/inv", "database/other", identity, 1, "c1", "", 15*time.Minute, now) {
		t.Fatal("forget must drop only its own key")
	}
}

func TestFailSinkExport(t *testing.T) {
	t.Parallel()

	rows := []struct {
		name       string
		err        error
		wantReason string
	}{
		{"transient", kollecterrors.Transient(errors.New("sink unavailable")), reasonExportFailed},
		{"spill required", kollecterrors.Terminal(fmt.Errorf("%w: too large", sink.ErrSpillRequired)), spillReasonSpillRequired},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			var tracker perSinkCoalesceTracker
			var outcome perSinkExportOutcome
			identity := sinkExportIdentity{inventoryUID: "inv-uid", sinkUID: "sink-uid", sinkGeneration: 1}
			now := time.Now()
			invKey, exportKey := "default/inv", "database/pg"
			tracker.record(invKey, exportKey, identity, 2, "c1", "", now)
			status := &kollectdevv1alpha1.InventorySinkExportStatus{Name: exportKey}

			failSinkExport(&tracker, &outcome, status, invKey, exportKey, 2, row.err)

			if outcome.FailedCount != 1 || outcome.FailedSink != exportKey || outcome.ExportErr == nil {
				t.Fatalf("outcome = failed %d sink %q err %v, want 1 %q non-nil",
					outcome.FailedCount, outcome.FailedSink, outcome.ExportErr, exportKey)
			}
			synced := apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
			if synced == nil || synced.Status != metav1.ConditionFalse || synced.Reason != row.wantReason {
				t.Fatalf("per-sink Synced = %+v, want False/%s", synced, row.wantReason)
			}
			if synced.ObservedGeneration != 2 || synced.Message != row.err.Error() {
				t.Fatalf("per-sink Synced generation %d message %q, want 2 %q",
					synced.ObservedGeneration, synced.Message, row.err.Error())
			}
			if tracker.shouldSkip(invKey, exportKey, identity, 2, "c1", "", 15*time.Minute, now) {
				t.Fatal("shouldSkip after failSinkExport = true, want false: a failed export clears the debounce state")
			}
		})
	}
}
