// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"sync"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

const conditionSinkSynced = "Synced"

type sinkCoalesceState struct {
	lastExport     time.Time
	lastChecksum   string
	lastGeneration int64
}

type perSinkCoalesceTracker struct {
	mu     sync.Mutex
	states map[string]*sinkCoalesceState
}

func (t *perSinkCoalesceTracker) key(invKey, sinkName string) string {
	return invKey + "\x00" + sinkName
}

func (t *perSinkCoalesceTracker) shouldSkip(
	invKey, sinkName string,
	generation int64,
	checksum string,
	interval time.Duration,
	now time.Time,
) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.states == nil {
		t.states = make(map[string]*sinkCoalesceState)
	}

	state := t.states[t.key(invKey, sinkName)]
	if state == nil || state.lastGeneration != generation {
		return false
	}
	if state.lastChecksum != checksum {
		return false
	}
	if interval == 0 {
		return true
	}
	if state.lastExport.IsZero() {
		return false
	}

	return now.Sub(state.lastExport) < interval
}

func (t *perSinkCoalesceTracker) record(invKey, sinkName string, generation int64, checksum string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.states == nil {
		t.states = make(map[string]*sinkCoalesceState)
	}

	k := t.key(invKey, sinkName)
	state := t.states[k]
	if state == nil {
		state = &sinkCoalesceState{}
		t.states[k] = state
	}

	state.lastExport = now
	state.lastChecksum = checksum
	state.lastGeneration = generation
}

func (t *perSinkCoalesceTracker) nextDue(
	invKey, sinkName string,
	interval time.Duration,
	now time.Time,
) time.Duration {
	if interval == 0 {
		return validation.ZeroIntervalWatchdog
	}

	t.mu.Lock()
	state := t.states[t.key(invKey, sinkName)]
	t.mu.Unlock()

	if state == nil || state.lastExport.IsZero() {
		return time.Second
	}

	remaining := interval - now.Sub(state.lastExport)
	if remaining < time.Second {
		return time.Second
	}

	return remaining
}

type perSinkExportOutcome struct {
	ExportedCount  int
	DebouncedCount int
	FailedSink     string
	ExportErr      error
	SinkExports    []kollectdevv1alpha1.InventorySinkExportStatus
	RequeueAfter   time.Duration
}

func mergeRequeueAfter(current, next time.Duration) time.Duration {
	if current == 0 || next < current {
		return next
	}
	return current
}

func setSinkExportSynced(
	status *kollectdevv1alpha1.InventorySinkExportStatus,
	generation int64,
	ok bool,
	reason, message string,
) {
	if status == nil {
		return
	}

	condStatus := metav1.ConditionTrue
	if !ok {
		condStatus = metav1.ConditionFalse
	}

	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionSinkSynced,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

func upsertSinkExportStatus(
	exports *[]kollectdevv1alpha1.InventorySinkExportStatus,
	name string,
) *kollectdevv1alpha1.InventorySinkExportStatus {
	for i := range *exports {
		if (*exports)[i].Name == name {
			return &(*exports)[i]
		}
	}

	*exports = append(*exports, kollectdevv1alpha1.InventorySinkExportStatus{Name: name})
	return &(*exports)[len(*exports)-1]
}

func aggregateInventorySync(
	conditions *[]metav1.Condition,
	generation int64,
	exported, debounced, failed int,
) {
	switch {
	case failed > 0:
		setSyncedCondition(conditions, generation, false, "Progressing",
			fmt.Sprintf("%d sink(s) failed export", failed))
	case debounced > 0 && exported == 0:
		setSyncedCondition(conditions, generation, false, kollectdevv1alpha1.ReasonPartiallySynced,
			fmt.Sprintf("%d sink(s) debounced", debounced))
	case debounced > 0:
		setSyncedCondition(conditions, generation, false, kollectdevv1alpha1.ReasonPartiallySynced,
			fmt.Sprintf("%d/%d sinks debounced", debounced, exported+debounced))
	default:
		setSyncedCondition(conditions, generation, true, "Exported",
			fmt.Sprintf("exported to %d sink(s)", exported))
	}
}

func latestExportTime(exports []kollectdevv1alpha1.InventorySinkExportStatus) *metav1.Time {
	var latest *metav1.Time
	for i := range exports {
		t := exports[i].LastExportTime
		if t == nil {
			continue
		}
		if latest == nil || t.After(latest.Time) {
			latest = t
		}
	}
	return latest
}
