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
	crbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/spoke"
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

	sinkCoalesce perSinkCoalesceTracker
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

	if !inv.DeletionTimestamp.IsZero() {
		return r.finalizeInventoryDeletion(ctx, &inv)
	}

	if err := r.ensureInventoryFinalizer(ctx, &inv); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	if inv.Spec.Suspend {
		return ctrl.Result{}, nil
	}

	itemCount := 0
	if r.Store != nil {
		itemCount = r.Store.CountForNamespace(inv.Namespace)
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

	items := r.Store.SnapshotNamespace(inv.Namespace)
	fingerprint, err := export.ItemsFingerprint(items)
	if err != nil {
		return ctrl.Result{}, err
	}

	if totalInventorySinkRefs(&inv) > 0 {
		if outcome, allDebounced := r.previewAllSinksDebounced(&inv, req.String(), fingerprint); allDebounced {
			metrics.ExportDebouncedTotal.WithLabelValues("KollectInventory").Add(float64(outcome.DebouncedCount))
			itemCount = len(items)

			return r.updateStatus(ctx, &inv, itemCount, outcome)
		}
	}

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

	itemCount = r.Store.CountForNamespace(inv.Namespace)

	if err := spoke.TryPublishReport(ctx, r.Store, &inv); err != nil {
		log.Error(err, "spoke hub publish", "inventory", inv.Name, "namespace", inv.Namespace)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, "SpokePublishFailed", err.Error())
		recordWarning(r.Recorder, &inv, "SpokePublishFailed", err.Error())
	}

	if totalInventorySinkRefs(&inv) == 0 {
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no family sink refs configured")
		return r.updateStatus(ctx, &inv, itemCount, perSinkExportOutcome{RequeueAfter: r.exportDebounce(&inv)})
	}

	return r.applyInventoryExportOutcome(ctx, log, &inv, itemCount, req.String(), items, fingerprint)
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
		resolved, err := loadResolvedSink(ctx, r.Client, inv.Namespace, binding, false)
		status := upsertSinkExportStatus(&outcome.SinkExports, exportKey)
		if err != nil {
			setSinkExportSynced(status, inv.Generation, false, reasonExportFailed, err.Error())
			outcome.FailedCount++
			outcome.ExportErr = err
			outcome.FailedSink = exportKey
			continue
		}

		var sinkInterval *metav1.Duration
		if resolved.ExportMinInterval != nil {
			sinkInterval = resolved.ExportMinInterval.ExportMinInterval
		}
		interval := validation.ResolveSinkExportInterval(ref, sinkInterval, defaultInterval, scopeFloor)
		if r.sinkCoalesce.shouldSkip(invKey, exportKey, inv.Generation, checksum, interval, now) {
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

			err := sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
				Ctx:           ctx,
				Client:        r.Client,
				Registry:      r.Registry,
				SinkNamespace: sink.SinkNamespaceForResolved(job.resolved, inv.Namespace),
				SinkName:      job.binding.Name,
				ObjectPath:    objectPath,
				Envelope:      envelope,
				SinkSpec:      job.resolved.Spec,
			})

			mu.Lock()
			defer mu.Unlock()

			exportKey := sinkExportKey(job.binding)
			if err != nil {
				log.Error(err, "export failed", "sink", exportKey)
				outcome.FailedCount++
				outcome.ExportErr = err
				outcome.FailedSink = exportKey
				setSinkExportSynced(job.status, inv.Generation, false, reasonExportFailed, err.Error())

				return
			}

			r.sinkCoalesce.record(invKey, exportKey, inv.Generation, checksum, now)
			exportTime := metav1.Now()
			job.status.LastExportTime = &exportTime
			job.status.LastChecksum = checksum
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
		if resolved, err := loadResolvedSink(context.Background(), r.Client, inv.Namespace, binding, false); err == nil {
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

	targetPredicate := predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
		predicate.LabelChangedPredicate{},
	)

	invBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectInventory{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectTarget{},
			handler.EnqueueRequestsFromMapFunc(r.mapTargetToInventories),
			crbuilder.WithPredicates(targetPredicate),
		).
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

	var list kollectdevv1alpha1.KollectInventoryList
	if err := r.List(ctx, &list, client.InNamespace(sinkNS)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list inventories for sink watch mapping",
			"sink", sinkName, "namespace", sinkNS, "family", family)
		metrics.WatchMapListErrorsTotal.WithLabelValues("KollectInventory", "sink").Inc()

		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		for _, binding := range inventorySinkBindings(&list.Items[i]) {
			if binding.Family == family && binding.Name == sinkName {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
				})

				break
			}
		}
	}

	return reqs
}

func (r *KollectInventoryReconciler) mapTargetToInventories(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	target, ok := obj.(*kollectdevv1alpha1.KollectTarget)
	if !ok {
		return nil
	}

	var list kollectdevv1alpha1.KollectInventoryList
	if err := r.List(ctx, &list, client.InNamespace(target.Namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list inventories for target watch mapping",
			"target", target.Name, "namespace", target.Namespace)
		metrics.WatchMapListErrorsTotal.WithLabelValues("KollectInventory", "target").Inc()

		return nil
	}

	return inventoriesInNamespace(ctx, r.Client, target.Namespace)
}
