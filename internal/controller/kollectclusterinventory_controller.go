// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/aggregate"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/validation"
)

// KollectClusterInventoryReconciler rolls up cluster targets and exports to namespaced sinks.
type KollectClusterInventoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    *collect.Store
	Engine   *collect.Engine
	Registry *sink.Registry
	Options  RuntimeOptions
	Recorder record.EventRecorder

	mu             sync.Mutex
	exportCoalesce map[string]*aggregate.ExportCoalesce
}

//nolint:lll // kubebuilder rbac marker length
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile aggregates cluster target rows and exports rollup payload to configured sinks.
func (r *KollectClusterInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectclusterinventory")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var inv kollectdevv1alpha1.KollectClusterInventory
	if err := r.Get(ctx, req.NamespacedName, &inv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if inv.Spec.Suspend {
		return ctrl.Result{}, nil
	}

	targets, err := r.selectedClusterTargets(ctx, &inv)
	if err != nil {
		retErr = err
		return ctrl.Result{}, err
	}

	if len(targets) == 0 {
		return r.setDegraded(ctx, &inv, "NoTargets", "no KollectClusterTarget objects matched")
	}

	itemCount, degradedTargets := r.rollupCounts(targets)
	if len(degradedTargets) > 0 {
		msg := fmt.Sprintf("%d target(s) not Ready: %v", len(degradedTargets), degradedTargets)
		return r.setDegraded(ctx, &inv, "TargetDegraded", msg)
	}

	sinkNS := inv.Spec.SinkNamespace
	if sinkNS == "" {
		sinkNS = sink.DefaultSecretNamespace
	}

	if len(inv.Spec.SinkRefs) > 0 {
		sinkOK, sinkReason, sinkMsg := checkClusterInventorySinksReachable(ctx, r.Client, sinkNS, inv.Spec.SinkRefs)
		setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, sinkOK, sinkReason, sinkMsg)
		if !sinkOK {
			recordWarning(r.Recorder, &inv, sinkReason, sinkMsg)
			return r.setDegraded(ctx, &inv, sinkReason, sinkMsg)
		}
	} else {
		setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, true, "NoSinksConfigured", "no sinkRefs configured")
	}

	if r.Store == nil || r.Engine == nil {
		return r.updateStatus(ctx, &inv, len(targets), itemCount, nil)
	}

	result, err := r.reconcileRollupExport(ctx, req, &inv, targets, sinkNS, log)
	if err != nil {
		retErr = err
	}
	return result, err
}

func (r *KollectClusterInventoryReconciler) reconcileRollupExport(
	ctx context.Context,
	req ctrl.Request,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targets []kollectdevv1alpha1.KollectClusterTarget,
	sinkNS string,
	log logr.Logger,
) (ctrl.Result, error) {
	payload, err := r.marshalRollupPayload(targets)
	if err != nil {
		return ctrl.Result{}, err
	}

	if limit := validation.MaxExportBytesGlobal(); limit > 0 && int64(len(payload)) > limit {
		msg := fmt.Sprintf("export payload %d bytes exceeds cap %d", len(payload), limit)
		metrics.SinkErrorsTotal.WithLabelValues("payload_too_large").Inc()

		return r.setDegraded(ctx, inv, "PayloadTooLarge", msg)
	}

	key := req.String()

	if r.shouldDebounce(inv, key, payload) {
		debounce := r.exportDebounce(inv)
		delay := debounce - time.Since(r.lastExportTime(key))
		if delay < time.Second {
			delay = time.Second
		}

		return ctrl.Result{RequeueAfter: delay}, nil
	}

	itemCount := r.countRollupItems(targets)

	if len(inv.Spec.SinkRefs) == 0 {
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no sinkRefs configured")
		return r.updateStatus(ctx, inv, len(targets), itemCount, nil)
	}

	if r.Registry == nil {
		return r.setDegraded(ctx, inv, "ExportUnavailable", "sink registry is not configured")
	}

	var exportErr error
	for _, sinkName := range inv.Spec.SinkRefs {
		if err := r.exportToSink(ctx, inv, sinkNS, sinkName, payload); err != nil {
			log.Error(err, "cluster export failed", "sink", sinkName)
			exportErr = err
		}
	}

	if exportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectClusterInventory", kollecterrors.ClassOf(exportErr)).Inc()
		reason := "Progressing"
		if kollecterrors.IsTerminal(exportErr) {
			reason = kollectdevv1alpha1.ReasonExportTerminal
		}
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, exportErr)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, exportErr.Error())
		recordWarning(r.Recorder, inv, reason, exportErr.Error())

		result, err := r.setDegraded(ctx, inv, reason, exportErr.Error())
		if kollecterrors.IsTerminal(exportErr) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	r.recordExport(inv, key, payload)

	return r.updateStatus(ctx, inv, len(targets), itemCount, nil)
}

func (r *KollectClusterInventoryReconciler) exportDebounce(
	inv *kollectdevv1alpha1.KollectClusterInventory,
) time.Duration {
	fallback := DefaultRuntimeOptions().ExportDebounce
	if r.Options.ExportDebounce > 0 {
		fallback = r.Options.ExportDebounce
	}

	return validation.ClusterExportMinIntervalFor(&inv.Spec, fallback)
}

func (r *KollectClusterInventoryReconciler) collectRollupItems(
	targets []kollectdevv1alpha1.KollectClusterTarget,
) []collect.Item {
	var items []collect.Item
	for i := range targets {
		ct := &targets[i]
		for _, ns := range r.Engine.NamespacesForClusterTarget(ct.Name) {
			items = append(items, r.Store.SnapshotTarget(ns, ct.Name)...)
		}
	}

	return aggregate.MergeRows(items, aggregate.DedupeByResourceUID)
}

func (r *KollectClusterInventoryReconciler) marshalRollupPayload(
	targets []kollectdevv1alpha1.KollectClusterTarget,
) ([]byte, error) {
	return json.Marshal(r.collectRollupItems(targets))
}

func (r *KollectClusterInventoryReconciler) countRollupItems(
	targets []kollectdevv1alpha1.KollectClusterTarget,
) int {
	return len(r.collectRollupItems(targets))
}

func (r *KollectClusterInventoryReconciler) exportToSink(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	sinkNS, sinkName string,
	payload []byte,
) error {
	var ks kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, client.ObjectKey{Namespace: sinkNS, Name: sinkName}, &ks); err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load KollectSink %q: %w", sinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(sinkErrorReason(err)).Inc()

		return err
	}

	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, ks.Spec, sinkNS)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(sinkErrorReason(err)).Inc()

		return err
	}

	backend, err := r.Registry.NewBackend(ks.Spec, buildCtx)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(sinkErrorReason(err)).Inc()

		return err
	}

	objectPath := fmt.Sprintf("inventory/cluster/%s.json", inv.Name)

	start := time.Now()
	err = backend.Export(ctx, payload, objectPath)
	elapsed := time.Since(start).Seconds()
	metrics.ExportDurationSeconds.WithLabelValues(ks.Spec.Type).Observe(elapsed)
	metrics.ExportBytesTotal.WithLabelValues(ks.Spec.Type).Add(float64(len(payload)))

	if err != nil {
		reason := sinkErrorReason(err)
		metrics.SinkErrorsTotal.WithLabelValues(reason).Inc()

		return kollecterrors.Transient(fmt.Errorf("export to %q: %w", sinkName, err))
	}

	return nil
}

func (r *KollectClusterInventoryReconciler) shouldDebounce(
	inv *kollectdevv1alpha1.KollectClusterInventory,
	key string,
	payload []byte,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.exportCoalesce == nil {
		r.exportCoalesce = make(map[string]*aggregate.ExportCoalesce)
	}

	return r.exportCoalesce[key].ShouldSkip(
		time.Now(),
		r.exportDebounce(inv),
		inv.Generation,
		payload,
	)
}

func (r *KollectClusterInventoryReconciler) recordExport(
	inv *kollectdevv1alpha1.KollectClusterInventory,
	key string,
	payload []byte,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.exportCoalesce == nil {
		r.exportCoalesce = make(map[string]*aggregate.ExportCoalesce)
	}

	c := r.exportCoalesce[key]
	if c == nil {
		c = &aggregate.ExportCoalesce{}
		r.exportCoalesce[key] = c
	}

	c.RecordExport(time.Now(), inv.Generation, payload)
}

func (r *KollectClusterInventoryReconciler) lastExportTime(key string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c := r.exportCoalesce[key]; c != nil {
		return c.LastExport
	}

	return time.Time{}
}

func (r *KollectClusterInventoryReconciler) selectedClusterTargets(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) ([]kollectdevv1alpha1.KollectClusterTarget, error) {
	var list kollectdevv1alpha1.KollectClusterTargetList
	if err := r.List(ctx, &list); err != nil {
		return nil, err
	}

	invSel, err := metav1.LabelSelectorAsSelector(inv.Spec.NamespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("namespaceSelector: %w", err)
	}

	targetSel, err := targetSelectorFor(inv)
	if err != nil {
		return nil, err
	}

	selected := make([]kollectdevv1alpha1.KollectClusterTarget, 0)
	for i := range list.Items {
		ct := list.Items[i]
		if !targetIncluded(inv, &ct) {
			continue
		}

		if targetSel != nil && !targetSel.Matches(labels.Set(ct.Labels)) {
			continue
		}

		if !clusterTargetMatchesInventoryNS(ctx, r.Client, &ct, invSel) {
			continue
		}

		selected = append(selected, ct)
	}

	return selected, nil
}

func targetSelectorFor(inv *kollectdevv1alpha1.KollectClusterInventory) (labels.Selector, error) {
	if inv.Spec.TargetSelector == nil {
		return labels.Everything(), nil
	}

	return metav1.LabelSelectorAsSelector(inv.Spec.TargetSelector)
}

func targetIncluded(inv *kollectdevv1alpha1.KollectClusterInventory, ct *kollectdevv1alpha1.KollectClusterTarget) bool {
	if len(inv.Spec.TargetRefs) == 0 {
		return true
	}

	for _, ref := range inv.Spec.TargetRefs {
		if ref == ct.Name {
			return true
		}
	}

	return false
}

func clusterTargetMatchesInventoryNS(
	ctx context.Context,
	c client.Client,
	ct *kollectdevv1alpha1.KollectClusterTarget,
	invSel labels.Selector,
) bool {
	var nsList corev1.NamespaceList
	if err := c.List(ctx, &nsList); err != nil {
		return false
	}

	targetSel, err := metav1.LabelSelectorAsSelector(ct.Spec.NamespaceSelector)
	if err != nil {
		return false
	}

	for i := range nsList.Items {
		ns := &nsList.Items[i]
		nsLabels := labels.Set(ns.Labels)
		if targetSel.Matches(nsLabels) && invSel.Matches(nsLabels) {
			return true
		}
	}

	return false
}

func (r *KollectClusterInventoryReconciler) rollupCounts(
	targets []kollectdevv1alpha1.KollectClusterTarget,
) (itemCount int, degraded []string) {
	for i := range targets {
		ct := &targets[i]
		if !clusterTargetReady(ct) {
			degraded = append(degraded, ct.Name)
		}
	}

	if r.Engine != nil && r.Store != nil {
		return len(r.collectRollupItems(targets)), degraded
	}

	for i := range targets {
		ct := &targets[i]
		if r.Engine != nil {
			for _, ns := range r.Engine.NamespacesForClusterTarget(ct.Name) {
				itemCount += r.Engine.ItemCount(ns, ct.Name)
			}
		}
	}

	return itemCount, degraded
}

func clusterTargetReady(ct *kollectdevv1alpha1.KollectClusterTarget) bool {
	cond := apimeta.FindStatusCondition(ct.Status.Conditions, conditionReady)
	return cond != nil && cond.Status == metav1.ConditionTrue
}

func checkClusterInventorySinksReachable(
	ctx context.Context,
	c client.Client,
	sinkNamespace string,
	sinkRefs []string,
) (bool, string, string) {
	for _, name := range sinkRefs {
		var ks kollectdevv1alpha1.KollectSink
		key := client.ObjectKey{Namespace: sinkNamespace, Name: name}
		if err := c.Get(ctx, key, &ks); err != nil {
			if apierrors.IsNotFound(err) {
				return false, "SinkNotFound", fmt.Sprintf("KollectSink %q not found in namespace %q", name, sinkNamespace)
			}

			return false, "SinkLookupFailed", err.Error()
		}
	}

	return true, "SinksReachable", fmt.Sprintf("%d sink(s) reachable in %q", len(sinkRefs), sinkNamespace)
}

func (r *KollectClusterInventoryReconciler) setDegraded(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	reason, message string,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionReady)
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
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, nil
}

func (r *KollectClusterInventoryReconciler) updateStatus(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targetCount, itemCount int,
	exportErr error,
) (ctrl.Result, error) {
	inv.Status.ObservedGeneration = inv.Generation
	inv.Status.TargetCount = targetCount
	inv.Status.ItemCount = itemCount

	if exportErr == nil && len(inv.Spec.SinkRefs) > 0 {
		now := metav1.Now()
		inv.Status.LastExportTime = &now
		apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, nil)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "Exported",
			fmt.Sprintf("exported %d item(s) from %d target(s) to %d sink(s)", itemCount, targetCount, len(inv.Spec.SinkRefs)))
		recordNormal(r.Recorder, inv, "ExportSucceeded",
			fmt.Sprintf("exported %d item(s) from %d target(s) to %d sink(s)", itemCount, targetCount, len(inv.Spec.SinkRefs)))
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionExportSucceeded,
			Status:             metav1.ConditionTrue,
			Reason:             "Exported",
			Message:            fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, len(inv.Spec.SinkRefs)),
			ObservedGeneration: inv.Generation,
			LastTransitionTime: metav1.Now(),
		})
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Exported",
			Message:            fmt.Sprintf("rolled up %d target(s), %d item(s)", targetCount, itemCount),
			ObservedGeneration: inv.Generation,
			LastTransitionTime: metav1.Now(),
		})
	} else if exportErr == nil {
		apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
		msg := fmt.Sprintf("rolled up %d target(s), %d item(s)", targetCount, itemCount)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "RolledUp", msg)
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "RolledUp",
			Message:            msg,
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

	return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectClusterInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentClusterInventory)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentClusterInventory
	}

	if r.Recorder == nil {
		//nolint:staticcheck // SA1019: record API until events migration
		r.Recorder = mgr.GetEventRecorderFor("kollectclusterinventory-controller")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectClusterInventory{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectClusterTarget{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterTargetToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapSinkToClusterInventories),
		).
		Named("kollectclusterinventory").
		Complete(r)
}

func (r *KollectClusterInventoryReconciler) mapClusterTargetToInventories(
	ctx context.Context,
	_ client.Object,
) []reconcile.Request {
	var list kollectdevv1alpha1.KollectClusterInventoryList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: list.Items[i].Name},
		})
	}

	return reqs
}

func (r *KollectClusterInventoryReconciler) mapSinkToClusterInventories(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	sinkObj, ok := obj.(*kollectdevv1alpha1.KollectSink)
	if !ok {
		return nil
	}

	var list kollectdevv1alpha1.KollectClusterInventoryList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	sinkNS := sinkObj.Namespace
	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		inv := &list.Items[i]
		invSinkNS := inv.Spec.SinkNamespace
		if invSinkNS == "" {
			invSinkNS = sink.DefaultSecretNamespace
		}

		if invSinkNS != sinkNS {
			continue
		}

		for _, ref := range inv.Spec.SinkRefs {
			if ref == sinkObj.Name {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: inv.Name},
				})

				break
			}
		}
	}

	return reqs
}
