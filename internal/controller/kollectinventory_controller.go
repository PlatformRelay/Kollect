// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
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
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
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

	sinkNames := inv.Spec.SinkRefs.Names()
	sinkOK, sinkReason, sinkMsg := checkInventorySinksReachable(ctx, r.Client, inv.Namespace, sinkNames)
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

	payload, err := r.Store.MarshalNamespaceExport(inv.Namespace, collect.ExportMetadata{
		Generation: inv.Generation,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	gate, err := assessExportSpill(
		ctx, r.Client, log, int64(len(payload)), r.maxExportBytes(&inv), inv.Namespace, sinkNames,
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
		log.Error(err, "spoke hub publish")
	}

	if len(inv.Spec.SinkRefs) == 0 {
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no sinkRefs configured")
		return r.updateStatus(ctx, &inv, itemCount, perSinkExportOutcome{RequeueAfter: r.exportDebounce(&inv)})
	}

	outcome := r.exportToSinks(ctx, log, &inv, req.String(), items, fingerprint)
	if outcome.ExportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectInventory", kollecterrors.ClassOf(outcome.ExportErr)).Inc()
		reason := reasonProgressing
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			reason = kollectdevv1alpha1.ReasonExportTerminal
		}
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, outcome.ExportErr)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, outcome.ExportErr.Error())
		recordWarning(r.Recorder, &inv, reason, outcome.ExportErr.Error())

		result, err := r.setInventoryDegraded(ctx, &inv, itemCount, reason, outcome.ExportErr.Error())
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	return r.updateStatus(ctx, &inv, itemCount, outcome)
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

	var outcome perSinkExportOutcome
	outcome.RequeueAfter = defaultInterval

	for _, ref := range inv.Spec.SinkRefs {
		sinkObj, err := r.loadSink(ctx, inv.Namespace, ref.Name)
		if err != nil {
			outcome.ExportErr = err
			outcome.FailedSink = ref.Name
			return outcome
		}

		interval := validation.ResolveSinkExportInterval(ref, sinkObj, defaultInterval, scopeFloor)
		status := upsertSinkExportStatus(&outcome.SinkExports, ref.Name)

		if r.sinkCoalesce.shouldSkip(invKey, ref.Name, inv.Generation, checksum, interval, now) {
			outcome.DebouncedCount++
			setSinkExportSynced(status, inv.Generation, false, kollectdevv1alpha1.ReasonDebounced,
				fmt.Sprintf("next export in %s (interval %s, checksum unchanged)",
					r.sinkCoalesce.nextDue(invKey, ref.Name, interval, now).Round(time.Second),
					interval))
			nextDue := r.sinkCoalesce.nextDue(invKey, ref.Name, interval, now)
			outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter, nextDue)
			continue
		}

		if err := sink.RunExportItems(sink.ExportItemsRequest{
			Ctx:           ctx,
			Client:        r.Client,
			Registry:      r.Registry,
			SinkNamespace: inv.Namespace,
			SinkName:      ref.Name,
			ObjectPath:    fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name),
			Items:         items,
			Meta:          export.Metadata{Generation: inv.Generation},
		}); err != nil {
			log.Error(err, "export failed", "sink", ref.Name)
			outcome.ExportErr = err
			outcome.FailedSink = ref.Name
			setSinkExportSynced(status, inv.Generation, false, reasonExportFailed, err.Error())
			return outcome
		}

		r.sinkCoalesce.record(invKey, ref.Name, inv.Generation, checksum, now)
		exportTime := metav1.Now()
		status.LastExportTime = &exportTime
		status.LastChecksum = checksum
		setSinkExportSynced(status, inv.Generation, true, "Exported", "export completed")
		outcome.ExportedCount++
		outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter, validation.RequeueAfterForZeroInterval(interval))
	}

	return outcome
}

type logrLogger interface {
	Error(err error, msg string, keysAndValues ...any)
}

func (r *KollectInventoryReconciler) loadSink(
	ctx context.Context,
	namespace, name string,
) (*kollectdevv1alpha1.KollectSink, error) {
	var ks kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &ks); err != nil {
		return nil, fmt.Errorf("get KollectSink %q: %w", name, err)
	}
	return &ks, nil
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

	if outcome.ExportErr == nil && len(inv.Spec.SinkRefs) > 0 {
		if latest := latestExportTime(outcome.SinkExports); latest != nil {
			inv.Status.LastExportTime = latest
		}
		apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, nil)
		aggregateInventorySync(&inv.Status.Conditions, inv.Generation,
			outcome.ExportedCount, outcome.DebouncedCount, 0)
		if outcome.ExportedCount > 0 {
			recordNormal(r.Recorder, inv, "ExportSucceeded",
				fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, outcome.ExportedCount))
		}
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Exported",
			Message:            fmt.Sprintf("exported %d item(s) across %d sink(s)", itemCount, len(inv.Spec.SinkRefs)),
			ObservedGeneration: inv.Generation,
			LastTransitionTime: metav1.Now(),
		})
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

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectInventory{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectTarget{},
			handler.EnqueueRequestsFromMapFunc(r.mapTargetToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapSinkToInventories),
		).
		Named("kollectinventory").
		Complete(r)
}

func (r *KollectInventoryReconciler) mapSinkToInventories(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	sinkObj, ok := obj.(*kollectdevv1alpha1.KollectSink)
	if !ok {
		return nil
	}

	var list kollectdevv1alpha1.KollectInventoryList
	if err := r.List(ctx, &list, client.InNamespace(sinkObj.Namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list inventories for sink watch mapping",
			"sink", sinkObj.Name, "namespace", sinkObj.Namespace)

		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		for _, ref := range list.Items[i].Spec.SinkRefs {
			if ref.Name == sinkObj.Name {
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

		return nil
	}

	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
		})
	}

	return reqs
}
