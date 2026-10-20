// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/redact"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/validation"
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

// sinkExportIdentity names the objects a sink export went to (SDC-4): the reconciled inventory
// object and the family sink object the binding resolved to. The tracker key (inventory
// namespace/name, sink export key) survives a delete and recreate of either object, and a sink
// spec edit bumps only the sink's metadata.generation, so the debounce baseline compares these
// too.
type sinkExportIdentity struct {
	inventoryUID   types.UID
	sinkUID        types.UID
	sinkGeneration int64
}

// newSinkExportIdentity builds the identity of an export from the inventory's UID and the
// resolved sink; a nil sink leaves the sink fields empty.
func newSinkExportIdentity(inventoryUID types.UID, resolved *sink.ResolvedSink) sinkExportIdentity {
	identity := sinkExportIdentity{inventoryUID: inventoryUID}
	if resolved != nil {
		identity.sinkUID = resolved.UID
		identity.sinkGeneration = resolved.Generation
	}

	return identity
}

type sinkCoalesceState struct {
	identity        sinkExportIdentity
	lastExport      time.Time
	lastChecksum    string
	lastGeneration  int64
	lastRequestedAt string
	// lastSkipped is when a 0s binding last skipped an unchanged payload. Such a binding never
	// records again while it stays unchanged, yet its watchdog reconciles keep consulting the
	// state, so pruning counts those skips as activity (SCR-7: no re-export after the TTL).
	lastSkipped time.Time
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
	identity sinkExportIdentity,
	generation int64,
	checksum, requestedAt string,
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
	// SDC-4: the same names can point at another inventory or sink object (deleted and
	// recreated) or at an edited sink spec; any of those re-exports.
	if state.identity != identity {
		return false
	}
	if state.lastChecksum != checksum {
		return false
	}
	// ERA-1 third axis: the manual re-export trigger. Any change of the
	// kollect.dev/requestedAt value — absence→present, present→absence, or a
	// different value — invalidates this binding's debounce once (checked
	// before the zero-interval early return so an interval==0 binding still
	// re-exports). record() pins the new value, so the steady-state debounce
	// resumes after the forced export.
	if state.lastRequestedAt != requestedAt {
		return false
	}
	if interval == 0 {
		state.lastSkipped = now

		return true
	}
	if state.lastExport.IsZero() {
		return false
	}

	return now.Sub(state.lastExport) < interval
}

func (t *perSinkCoalesceTracker) record(
	invKey, sinkName string,
	identity sinkExportIdentity,
	generation int64,
	checksum, requestedAt string,
	now time.Time,
) {
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

	state.identity = identity
	state.lastExport = now
	state.lastChecksum = checksum
	state.lastGeneration = generation
	state.lastRequestedAt = requestedAt
}

// forget drops the debounce state of one sink binding (SDC-4): after a failed export the sink is
// not debounced until an export to it succeeds again, also when the inputs return to those of
// the last successful export.
func (t *perSinkCoalesceTracker) forget(invKey, sinkName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.states, t.key(invKey, sinkName))
}

// pruneStaleLocked evicts entries that haven't recorded an export, nor skipped
// one under a 0s interval, in coalesceStateTTL. Callers must hold t.mu. Triggered opportunistically on
// every record() (AR-11) so the map doesn't grow unbounded for long-lived
// controller processes as inventories/sinks churn — entries for deleted
// inventories/sinks stop receiving record() calls and age out the next time
// any other key is recorded.
func (t *perSinkCoalesceTracker) pruneStaleLocked(now time.Time) {
	for k, state := range t.states {
		if state == nil || now.Sub(latestTime(state.lastExport, state.lastSkipped)) > coalesceStateTTL {
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

// failSinkExport records a failed export of one sink in an export pass: the outcome counts the
// failure, the sink's entry reads Synced=False with the error's reason, and the sink's debounce
// state is dropped.
func failSinkExport(
	tracker *perSinkCoalesceTracker,
	outcome *perSinkExportOutcome,
	status *kollectdevv1alpha1.InventorySinkExportStatus,
	invKey, exportKey string,
	generation int64,
	err error,
) {
	outcome.addSinkFailure(exportKey, err)
	setSinkExportSynced(status, generation, false, sinkExportFailureReason(err), err.Error())
	tracker.forget(invKey, exportKey)
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

// sinkExportFailureReason maps a per-sink export error to its status reason. A
// payload that needs an object-store spill but has none is reported as
// SpillRequired (the actionable operator signal) rather than the generic
// ExportFailed; every other failure keeps the existing reason (K-01).
func sinkExportFailureReason(err error) string {
	if errors.Is(err, sink.ErrSpillRequired) {
		return spillReasonSpillRequired
	}

	return reasonExportFailed
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
		Message:            redact.Text(message),
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

// setSinkExportDebounced marks a sink skipped as up to date (SDC-1): Synced=True, reason
// Debounced. The message names the effective interval and nothing time-dependent, so repeated
// debounced reconciles write the same condition.
func setSinkExportDebounced(
	status *kollectdevv1alpha1.InventorySinkExportStatus,
	generation int64,
	interval time.Duration,
) {
	message := fmt.Sprintf("payload unchanged since the last export; re-exported every %s", interval)
	if interval == 0 {
		message = "payload unchanged since the last export; exports only on change (interval 0s)"
	}

	setSinkExportSynced(status, generation, true, kollectdevv1alpha1.ReasonDebounced, message)
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

// carryOverSinkExport seeds a rebuilt sinkExports entry from the previous entry of the same
// name: lastExportTime, lastChecksum, lastExportPaths and the conditions, deep-copied
// (SDC-5). outcome.SinkExports is rebuilt every reconcile and overwrites the inventory status
// wholesale, so without the seed a debounced or failed pass would erase the last successful
// export's record, including the paths the deletion-time cleanup needs as retraction evidence
// (K-28), and recreate the Synced condition with a new lastTransitionTime. Only a successful
// export replaces the record; a debounced or failed pass changes the condition only, and
// SetStatusCondition keeps lastTransitionTime while its status stays the same. Call it on the
// fresh entry before the pass decides anything for the sink, in the sequential loop over
// bindings (before any export goroutine starts).
func carryOverSinkExport(
	status *kollectdevv1alpha1.InventorySinkExportStatus,
	previous []kollectdevv1alpha1.InventorySinkExportStatus,
	exportKey string,
) {
	if status == nil {
		return
	}

	for i := range previous {
		if previous[i].Name != exportKey {
			continue
		}

		seed := previous[i].DeepCopy()
		status.LastExportTime = seed.LastExportTime
		status.LastChecksum = seed.LastChecksum
		status.LastExportPaths = seed.LastExportPaths
		status.Conditions = seed.Conditions

		return
	}
}

// maxRecordedExportPaths bounds the per-sink lastExportPaths status list (the
// CRD caps the field at the same size). A projection with more paths — in
// practice a tree-shaped per-resource export — is sampled to the first entries
// of the sorted set: its recorded paths land outside the document-side
// candidate paths either way, so the retention evidence survives sampling.
const maxRecordedExportPaths = 32

// recordExportPaths turns one successful export's written paths into the
// recorded status value: deduplicated and sorted for a deterministic sample,
// capped at maxRecordedExportPaths. An empty or nil write (capability skip,
// empty projection) keeps the previous recording — an export that wrote
// nothing did not invalidate the paths recorded before it.
func recordExportPaths(written, previous []string) []string {
	if len(written) == 0 {
		return previous
	}

	seen := make(map[string]struct{}, len(written))
	out := make([]string, 0, len(written))
	for _, p := range written {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)

	if len(out) > maxRecordedExportPaths {
		out = out[:maxRecordedExportPaths]
	}

	return out
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
	default:
		// SDC-2: a debounced sink is up to date, so an export pass and a debounce pass over the
		// same sinks write the same condition.
		setSyncedCondition(conditions, generation, true, reasonExported, sinksUpToDateMessage(exported, debounced))
	}
}

// sinksUpToDateMessage counts exported and debounced sinks as up to date out of the bound sinks
// of a pass in which no sink failed.
func sinksUpToDateMessage(exported, debounced int) string {
	upToDate := exported + debounced

	return fmt.Sprintf("%d/%d sink(s) up to date", upToDate, upToDate)
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

// latestTime returns the later of a and b.
func latestTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}

	return a
}
