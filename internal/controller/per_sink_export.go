// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/validation"
)

const conditionSinkSynced = "Synced"

// coalesceStateTTL bounds the lifetime of an idle tracker entry. The tracker
// is a long-lived field on each reconciler (kept for the life of the
// controller-manager process), so entries for inventories/sinks that have
// since been deleted must eventually be reclaimed or the map grows
// unbounded for long-lived clusters with churning targets.
//
// This must stay comfortably above validation.MaxExportInterval (24h): an
// entry can legitimately go up to one full export interval between
// record() calls, and pruning it early would make the tracker forget an
// active sink's debounce state, forcing an unnecessary re-export. 2x the
// max interval leaves headroom for reconcile jitter/backoff.
const coalesceStateTTL = 2 * validation.MaxExportInterval

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

	t.pruneStaleLocked(now)

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

// pruneStaleLocked evicts entries that haven't recorded an export in
// coalesceStateTTL. Callers must hold t.mu. Triggered opportunistically on
// every record() (AR-11) so the map doesn't grow unbounded for long-lived
// controller processes as inventories/sinks churn — entries for deleted
// inventories/sinks stop receiving record() calls and age out the next time
// any other key is recorded.
func (t *perSinkCoalesceTracker) pruneStaleLocked(now time.Time) {
	for k, state := range t.states {
		if state == nil || now.Sub(state.lastExport) > coalesceStateTTL {
			delete(t.states, k)
		}
	}
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
	FailedCount    int
	FailedSink     string
	ExportErr      error
	SinkExports    []kollectdevv1alpha1.InventorySinkExportStatus
	RequeueAfter   time.Duration

	failedSinks []string
	exportErrs  []error
}

// addSinkFailure accumulates a per-sink export failure (EC-P1-06): FailedSink
// names every failed sink and ExportErr aggregates all failure errors instead
// of last-write-wins.
func (o *perSinkExportOutcome) addSinkFailure(exportKey string, err error) {
	o.FailedCount++
	o.failedSinks = append(o.failedSinks, exportKey)
	o.exportErrs = append(o.exportErrs, err)
	o.FailedSink = strings.Join(o.failedSinks, ",")
	o.ExportErr = aggregateExportErrs(o.exportErrs)
}

// aggregateExportErrs joins per-sink failures into one classified error. The
// aggregate is terminal only when ALL components are terminal; otherwise it is
// transient so retry still happens. The explicit re-wrap is required because
// errors.As on a joined error returns the first ClassError in DFS order, which
// would misclassify mixed failures.
func aggregateExportErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	joined := errors.Join(errs...)
	for _, err := range errs {
		if !kollecterrors.IsTerminal(err) {
			return kollecterrors.Transient(joined)
		}
	}

	return kollecterrors.Terminal(joined)
}

func isTotalExportFailure(outcome perSinkExportOutcome) bool {
	return outcome.ExportErr != nil && outcome.ExportedCount == 0 && outcome.DebouncedCount == 0
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
	case failed > 0 && exported > 0:
		setSyncedCondition(conditions, generation, false, kollectdevv1alpha1.ReasonPartiallySynced,
			fmt.Sprintf("%d/%d sinks exported; %d failed", exported, exported+failed, failed))
	case failed > 0:
		setSyncedCondition(conditions, generation, false, reasonProgressing,
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
