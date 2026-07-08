// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/validation"
)

// KollectInventoryReconciler reconciles a KollectInventory object
type KollectInventoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    *collect.Store
	Registry *sink.Registry
	Options  RuntimeOptions
	Recorder record.EventRecorder

	sinkCoalesce       perSinkCoalesceTracker
	nsFingerprintCache namespaceFingerprintCache
}

// namespaceFingerprintCache memoizes the namespace content fingerprint
// against the Store's per-namespace mutation version (AR-10 / PERF-01
// remainder). Reconcile calls this every cycle; as long as nothing was
// upserted/removed for the namespace since the last call, the expensive
// SnapshotNamespace + ItemsFingerprint pair is skipped entirely.
type namespaceFingerprintCache struct {
	mu    sync.Mutex
	state map[string]namespaceFingerprintEntry
}

type namespaceFingerprintEntry struct {
	version     uint64
	fingerprint string
}

// getOrCompute returns the cached fingerprint for namespace if it was last
// computed at the given version; otherwise it calls compute, caches the
// result against version, and returns it.
func (c *namespaceFingerprintCache) getOrCompute(namespace string, version uint64, compute func() string) string {
	c.mu.Lock()
	if entry, ok := c.state[namespace]; ok && entry.version == version {
		c.mu.Unlock()

		return entry.fingerprint
	}
	c.mu.Unlock()

	fingerprint := compute()

	c.mu.Lock()
	if c.state == nil {
		c.state = make(map[string]namespaceFingerprintEntry)
	}
	c.state[namespace] = namespaceFingerprintEntry{version: version, fingerprint: fingerprint}
	c.mu.Unlock()

	return fingerprint
}

// cacheResultLabel maps the namespaceFingerprintCache compute outcome to the
// kollect_namespace_fingerprint_cache_total "result" label value.
func cacheResultLabel(computed bool) string {
	if computed {
		return "miss"
	}

	return "hit"
}

func (r *KollectInventoryReconciler) exportDebounce(inv *kollectdevv1alpha1.KollectInventory) time.Duration {
	return validation.ExportMinIntervalFor(&inv.Spec, 0)
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks;kollectdatabasesinks;kollecteventsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectscopes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile aggregates collected items in the namespace and exports to configured sinks.
func (r *KollectInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectinventory")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var inv kollectdevv1alpha1.KollectInventory
	if err := r.Get(ctx, req.NamespacedName, &inv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return guardReconcile(ctx, r.Recorder, &inv, func() (ctrl.Result, error) {
		if !inv.DeletionTimestamp.IsZero() {
			return r.finalizeInventoryDeletion(ctx, &inv)
		}

		if err := r.ensureInventoryFinalizer(ctx, &inv); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}

			return ctrl.Result{}, err
		}

		itemCount := 0
		if r.Store != nil {
			itemCount = r.Store.CountForNamespace(inv.Namespace)
		}

		if inv.Spec.Suspend {
			return r.setInventoryDegraded(ctx, &inv, itemCount, "Suspended", "spec.suspend is true")
		}

		checker := scopeCheck{client: r.Client, recorder: r.Recorder}
		if ok, reason, msg := checker.enforceInventory(ctx, &inv); !ok {
			return r.setInventoryDegraded(ctx, &inv, itemCount, reason, msg)
		}

		bindings := inventorySinkBindings(&inv)
		sinkOK, sinkReason, sinkMsg := checkInventorySinksReachable(ctx, r.Client, inv.Namespace, bindings)
		setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, sinkOK, sinkReason, sinkMsg)
		if !sinkOK {
			recordWarning(r.Recorder, &inv, sinkReason, sinkMsg)
			return r.setInventoryDegraded(ctx, &inv, itemCount, sinkReason, sinkMsg)
		}

		if r.Store == nil {
			return ctrl.Result{}, nil
		}

		// AR-10 (PERF-01 remainder): the namespace content fingerprint is only
		// needed to decide whether to skip export (all sinks debounced) or,
		// failing that, to build the actual export payload. As long as the
		// Store hasn't been mutated for this namespace since the last
		// reconcile, the cached fingerprint from that reconcile is still
		// exactly the value a fresh SnapshotNamespace + ItemsFingerprint
		// would produce, so the (O(n) item-copy + JSON-marshal + hash) work
		// is skipped in the common steady-state case.
		var items []collect.Item
		var itemsComputed bool
		var fingerprintErr error
		nsVersion := r.Store.NamespaceVersion(inv.Namespace)
		fingerprint := r.nsFingerprintCache.getOrCompute(inv.Namespace, nsVersion, func() string {
			itemsComputed = true
			items = r.Store.SnapshotNamespace(inv.Namespace)
			fp, err := export.ItemsFingerprint(items)
			fingerprintErr = err

			return fp
		})
		if fingerprintErr != nil {
			return ctrl.Result{}, fingerprintErr
		}

		metrics.NamespaceFingerprintCacheTotal.WithLabelValues("KollectInventory", cacheResultLabel(itemsComputed)).Inc()

		if totalInventorySinkRefs(&inv) > 0 {
			if outcome, allDebounced := r.previewAllSinksDebounced(&inv, req.String(), fingerprint); allDebounced {
				metrics.ExportDebouncedTotal.WithLabelValues("KollectInventory").Add(float64(outcome.DebouncedCount))

				return r.updateStatus(ctx, &inv, itemCount, outcome)
			}
		}

		if items == nil {
			items = r.Store.SnapshotNamespace(inv.Namespace)
		}

		if !hasSnapshotSinkBinding(bindings) {
			payload, err := r.Store.MarshalNamespaceExport(inv.Namespace, collect.ExportMetadata{
				Generation: inv.Generation,
			})
			if err != nil {
				return ctrl.Result{}, err
			}

			gate, err := assessExportSpill(
				ctx, r.Client, log, int64(len(payload)), r.maxExportBytes(&inv), inv.Namespace, bindings,
			)
			if err != nil {
				return ctrl.Result{}, err
			}
			if gate.degraded {
				recordSpillGateMetrics(gate)

				return r.setInventoryDegraded(ctx, &inv, itemCount, gate.reason, gate.message)
			}
		}

		itemCount = r.Store.CountForNamespace(inv.Namespace)

		if noteExportShardWarning(&inv.Status.Conditions, inv.Generation, itemCount) {
			metrics.ExportShardWarnTotal.Inc()
			recordWarning(r.Recorder, &inv, reasonExportShardWarn,
				fmt.Sprintf("%d rows in namespace — shard exports across multiple KollectInventory resources", itemCount))
		}

		if totalInventorySinkRefs(&inv) == 0 {
			setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no family sink refs configured")
			return r.updateStatus(ctx, &inv, itemCount, perSinkExportOutcome{RequeueAfter: r.exportDebounce(&inv)})
		}

		return r.applyInventoryExportOutcome(ctx, log, &inv, itemCount, req.String(), items, fingerprint)
	})
}

func (r *KollectInventoryReconciler) applyInventoryExportOutcome(
	ctx context.Context,
	log logrLogger,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	invKey string,
	items []collect.Item,
	fingerprint string,
) (ctrl.Result, error) {
	outcome := r.exportToSinks(ctx, log, inv, invKey, items, fingerprint)
	if isTotalExportFailure(outcome) {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectInventory", kollecterrors.ClassOf(outcome.ExportErr)).Inc()
		reason := reasonProgressing
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			reason = kollectdevv1alpha1.ReasonExportTerminal
		}
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, outcome.ExportErr)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, outcome.ExportErr.Error())
		recordWarning(r.Recorder, inv, reason, outcome.ExportErr.Error())

		result, err := r.setInventoryDegraded(ctx, inv, itemCount, reason, outcome.ExportErr.Error())
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	if outcome.ExportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectInventory", kollecterrors.ClassOf(outcome.ExportErr)).Inc()
		recordWarning(r.Recorder, inv, reasonExportFailed, outcome.ExportErr.Error())
	}

	return r.updateStatus(ctx, inv, itemCount, outcome)
}

func (r *KollectInventoryReconciler) exportToSinks(
	ctx context.Context,
	log logrLogger,
	inv *kollectdevv1alpha1.KollectInventory,
	invKey string,
	items []collect.Item,
	checksum string,
) perSinkExportOutcome {
	now := time.Now()
	defaultInterval := r.exportDebounce(inv)
	scopeFloor := r.scopeFloor(ctx, inv.Namespace)
	maxExportBytes := r.maxExportBytes(inv)

	snapshotParts, err := export.PartitionEnvelopes(items, export.Metadata{
		Generation: inv.Generation,
		ExportedAt: now.UTC(),
	}, maxExportBytes)
	if err != nil {
		var outcome perSinkExportOutcome
		outcome.ExportErr = kollecterrors.Terminal(err)
		outcome.RequeueAfter = defaultInterval

		return outcome
	}
	snapshotChecksum := export.PartitionsChecksum(snapshotParts)

	bindings := inventorySinkBindings(inv)
	var outcome perSinkExportOutcome
	outcome.RequeueAfter = defaultInterval
	outcome.SinkExports = make([]kollectdevv1alpha1.InventorySinkExportStatus, 0, len(bindings))

	type sinkJob struct {
		binding  kollectdevv1alpha1.InventorySinkBinding
		ref      kollectdevv1alpha1.InventorySinkRef
		resolved *sink.ResolvedSink
		interval time.Duration
		status   *kollectdevv1alpha1.InventorySinkExportStatus
	}

	jobs := make([]sinkJob, 0, len(bindings))
	for _, binding := range bindings {
		ref := binding.Ref
		exportKey := sinkExportKey(binding)
		resolved, resolvedErr := loadResolvedSink(ctx, r.Client, inv.Namespace, binding)
		status := upsertSinkExportStatus(&outcome.SinkExports, exportKey)
		if resolvedErr != nil {
			setSinkExportSynced(status, inv.Generation, false, reasonExportFailed, resolvedErr.Error())
			outcome.addSinkFailure(exportKey, resolvedErr)
			continue
		}

		var sinkInterval *metav1.Duration
		if resolved.ExportMinInterval != nil {
			sinkInterval = resolved.ExportMinInterval.ExportMinInterval
		}
		interval := validation.ResolveSinkExportInterval(ref, sinkInterval, defaultInterval, scopeFloor)
		sinkChecksum := checksum
		if binding.Family == kollectdevv1alpha1.SinkFamilySnapshot {
			sinkChecksum = snapshotChecksum
		}
		if r.sinkCoalesce.shouldSkip(invKey, exportKey, inv.Generation, sinkChecksum, interval, now) {
			outcome.DebouncedCount++
			metrics.ExportDebouncedTotal.WithLabelValues("KollectInventory").Inc()
			setSinkExportSynced(status, inv.Generation, false, kollectdevv1alpha1.ReasonDebounced,
				fmt.Sprintf("next export in %s (interval %s, checksum unchanged)",
					r.sinkCoalesce.nextDue(invKey, exportKey, interval, now).Round(time.Second),
					interval))
			nextDue := r.sinkCoalesce.nextDue(invKey, exportKey, interval, now)
			outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter, nextDue)
			continue
		}

		jobs = append(jobs, sinkJob{binding: binding, ref: ref, resolved: resolved, interval: interval, status: status})
	}

	if len(jobs) == 0 {
		return outcome
	}

	envelope, err := export.MarshalEnvelope(items, export.Metadata{
		Generation: inv.Generation,
		ExportedAt: now.UTC(),
	})
	if err != nil {
		outcome.ExportErr = err
		outcome.FailedCount = len(jobs)

		return outcome
	}

	objectPath := fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, job := range jobs {
		wg.Add(1)

		go func(job sinkJob) {
			defer wg.Done()

			sinkChecksum := checksum
			exportErr := error(nil)
			if job.binding.Family == kollectdevv1alpha1.SinkFamilySnapshot {
				sinkChecksum = snapshotChecksum
				for _, part := range snapshotParts {
					partPath := export.PartitionObjectPath(objectPath, part.Index, part.Total)
					exportErr = sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
						Ctx:           ctx,
						Client:        r.Client,
						Registry:      r.Registry,
						SinkNamespace: sink.SinkNamespaceForResolved(job.resolved, inv.Namespace),
						SinkName:      job.binding.Name,
						SinkUID:       job.resolved.UID,
						ObjectPath:    partPath,
						Envelope:      part.Envelope,
						SinkSpec:      job.resolved.Spec,
					})
					if exportErr != nil {
						break
					}
				}
			} else {
				exportErr = sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
					Ctx:           ctx,
					Client:        r.Client,
					Registry:      r.Registry,
					SinkNamespace: sink.SinkNamespaceForResolved(job.resolved, inv.Namespace),
					SinkName:      job.binding.Name,
					SinkUID:       job.resolved.UID,
					ObjectPath:    objectPath,
					Envelope:      envelope,
					SinkSpec:      job.resolved.Spec,
				})
			}

			mu.Lock()
			defer mu.Unlock()

			exportKey := sinkExportKey(job.binding)
			if exportErr != nil {
				log.Error(exportErr, "export failed", "sink", exportKey)
				outcome.addSinkFailure(exportKey, exportErr)
				setSinkExportSynced(job.status, inv.Generation, false, reasonExportFailed, exportErr.Error())

				return
			}

			r.sinkCoalesce.record(invKey, exportKey, inv.Generation, sinkChecksum, now)
			exportTime := metav1.Now()
			job.status.LastExportTime = &exportTime
			job.status.LastChecksum = sinkChecksum
			setSinkExportSynced(job.status, inv.Generation, true, "Exported", "export completed")
			outcome.ExportedCount++
			outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter,
				validation.RequeueAfterForZeroInterval(job.interval))
		}(job)
	}

	wg.Wait()

	return outcome
}

func (r *KollectInventoryReconciler) previewAllSinksDebounced(
	inv *kollectdevv1alpha1.KollectInventory,
	invKey, checksum string,
) (perSinkExportOutcome, bool) {
	bindings := inventorySinkBindings(inv)
	if len(bindings) == 0 {
		return perSinkExportOutcome{}, false
	}

	now := time.Now()
	defaultInterval := r.exportDebounce(inv)
	scopeFloor := r.scopeFloor(context.Background(), inv.Namespace)

	var outcome perSinkExportOutcome
	outcome.RequeueAfter = defaultInterval
	allDebounced := true

	for _, binding := range bindings {
		ref := binding.Ref
		exportKey := sinkExportKey(binding)
		status := upsertSinkExportStatus(&outcome.SinkExports, exportKey)
		interval := defaultInterval
		if resolved, err := loadResolvedSink(context.Background(), r.Client, inv.Namespace, binding); err == nil {
			var sinkInterval *metav1.Duration
			if resolved.ExportMinInterval != nil {
				sinkInterval = resolved.ExportMinInterval.ExportMinInterval
			}
			interval = validation.ResolveSinkExportInterval(ref, sinkInterval, defaultInterval, scopeFloor)
		}

		if r.sinkCoalesce.shouldSkip(invKey, exportKey, inv.Generation, checksum, interval, now) {
			outcome.DebouncedCount++
			setSinkExportSynced(status, inv.Generation, false, kollectdevv1alpha1.ReasonDebounced,
				fmt.Sprintf("next export in %s (interval %s, checksum unchanged)",
					r.sinkCoalesce.nextDue(invKey, exportKey, interval, now).Round(time.Second),
					interval))
			nextDue := r.sinkCoalesce.nextDue(invKey, exportKey, interval, now)
			outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter, nextDue)
			continue
		}

		allDebounced = false

		break
	}

	if !allDebounced || outcome.DebouncedCount != len(bindings) {
		return perSinkExportOutcome{}, false
	}

	return outcome, true
}

type logrLogger interface {
	Error(err error, msg string, keysAndValues ...any)
}

func (r *KollectInventoryReconciler) scopeFloor(ctx context.Context, namespace string) time.Duration {
	binding, err := scope.Load(ctx, r.Client, namespace)
	if err != nil || !binding.Enforced || binding.Scope == nil {
		return 0
	}
	return validation.ScopeMinExportInterval(binding.Scope)
}

func (r *KollectInventoryReconciler) maxExportBytes(inv *kollectdevv1alpha1.KollectInventory) int64 {
	if inv.Spec.MaxExportBytes != nil && *inv.Spec.MaxExportBytes > 0 {
		return *inv.Spec.MaxExportBytes
	}

	return validation.MaxExportBytesGlobal()
}

func (r *KollectInventoryReconciler) updateStatus(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	outcome perSinkExportOutcome,
) (ctrl.Result, error) {
	inv.Status.ObservedGeneration = inv.Generation
	inv.Status.ItemCount = itemCount
	inv.Status.SinkExports = outcome.SinkExports

	if totalInventorySinkRefs(inv) > 0 {
		if latest := latestExportTime(outcome.SinkExports); latest != nil {
			inv.Status.LastExportTime = latest
		}

		failed := outcome.FailedCount
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, outcome.ExportErr)
		aggregateInventorySync(&inv.Status.Conditions, inv.Generation,
			outcome.ExportedCount, outcome.DebouncedCount, failed)

		sinkCount := totalInventorySinkRefs(inv)
		switch {
		case failed == 0 && outcome.ExportErr == nil:
			apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
			if outcome.ExportedCount > 0 {
				recordNormal(r.Recorder, inv, "ExportSucceeded",
					fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, outcome.ExportedCount))
			}
			apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             "Exported",
				Message:            fmt.Sprintf("exported %d item(s) across %d sink(s)", itemCount, sinkCount),
				ObservedGeneration: inv.Generation,
				LastTransitionTime: metav1.Now(),
			})
		case outcome.ExportedCount > 0:
			apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
			apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             kollectdevv1alpha1.ReasonPartiallySynced,
				Message:            fmt.Sprintf("exported %d item(s) to %d/%d sink(s)", itemCount, outcome.ExportedCount, sinkCount),
				ObservedGeneration: inv.Generation,
				LastTransitionTime: metav1.Now(),
			})
		}
	}

	if err := r.Status().Update(ctx, inv); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	requeue := outcome.RequeueAfter
	if requeue <= 0 {
		requeue = r.exportDebounce(inv)
	}

	return ctrl.Result{RequeueAfter: requeue}, nil
}

func (r *KollectInventoryReconciler) setInventoryDegraded(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	reason, message string,
) (ctrl.Result, error) {
	inv.Status.ItemCount = itemCount
	inv.Status.ObservedGeneration = inv.Generation
	setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, message)
	apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: inv.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, inv); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, nil
}

func hasSnapshotSinkBinding(bindings []kollectdevv1alpha1.InventorySinkBinding) bool {
	for i := range bindings {
		if bindings[i].Family == kollectdevv1alpha1.SinkFamilySnapshot {
			return true
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentInventory)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentInventory
	}

	if r.Recorder == nil {
		//nolint:staticcheck // SA1019: record API until events migration
		r.Recorder = mgr.GetEventRecorderFor("kollectinventory-controller")
	}

	// AR-09: index KollectInventory by its sink bindings so sink-watch mappers can do an indexed
	// MatchingFields lookup instead of listing + filtering every inventory in the namespace.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &kollectdevv1alpha1.KollectInventory{}, inventorySinkFieldIndex, indexInventorySinkBindings,
	); err != nil {
		return fmt.Errorf("index %s on KollectInventory: %w", inventorySinkFieldIndex, err)
	}

	invBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectInventory{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectSnapshotSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapSnapshotSinkToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectDatabaseSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapDatabaseSinkToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectEventSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapEventSinkToInventories),
		).
		Named("kollectinventory")

	if r.Store != nil {
		invBuilder = invBuilder.WatchesRawSource(newInventoryStoreSource(r.Store, r.Client))
	}

	return invBuilder.Complete(r)
}

func (r *KollectInventoryReconciler) mapSnapshotSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilySnapshot)
}

func (r *KollectInventoryReconciler) mapDatabaseSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilyDatabase)
}

func (r *KollectInventoryReconciler) mapEventSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilyEvent)
}

func (r *KollectInventoryReconciler) mapFamilySinkToInventories(
	ctx context.Context,
	obj client.Object,
	family string,
) []reconcile.Request {
	sinkName := obj.GetName()
	sinkNS := obj.GetNamespace()

	indexKey := sinkExportKey(kollectdevv1alpha1.InventorySinkBinding{Family: family, Name: sinkName})

	var list kollectdevv1alpha1.KollectInventoryList
	if err := r.List(ctx, &list,
		client.InNamespace(sinkNS),
		client.MatchingFields{inventorySinkFieldIndex: indexKey},
	); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list inventories for sink watch mapping",
			"sink", sinkName, "namespace", sinkNS, "family", family)
		metrics.WatchMapListErrorsTotal.WithLabelValues("KollectInventory", "sink").Inc()

		return nil
	}

	// The field index already restricts the list to inventories binding this family/name in this
	// namespace (AR-09), so every returned item is a match — no in-memory filter needed.
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
		})
	}

	return reqs
}
