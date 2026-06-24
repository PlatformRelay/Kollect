// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
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
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/validation"
)

const clusterTargetBootstrapWindow = 5 * time.Minute

// KollectClusterInventoryReconciler rolls up cluster targets and exports to namespaced sinks.
type KollectClusterInventoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    *collect.Store
	Engine   *collect.Engine
	Registry *sink.Registry
	Options  RuntimeOptions
	Recorder record.EventRecorder

	sinkCoalesce perSinkCoalesceTracker
}

//nolint:lll // kubebuilder rbac marker length
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks;kollectdatabasesinks;kollecteventsinks,verbs=get;list;watch
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

	return guardReconcile(ctx, r.Recorder, &inv, func() (ctrl.Result, error) {
		if !inv.DeletionTimestamp.IsZero() {
			return r.finalizeClusterInventoryDeletion(ctx, &inv)
		}

		if err := r.ensureClusterInventoryFinalizer(ctx, &inv); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}

			return ctrl.Result{}, err
		}

		if inv.Spec.Suspend {
			return r.setDegraded(ctx, &inv, "Suspended", "spec.suspend is true")
		}

		inventoryNamespaces, err := r.selectedInventoryNamespaces(ctx, &inv)
		if err != nil {
			retErr = err
			return ctrl.Result{}, err
		}
		selectedNamespaceNames := namespaceNameSet(inventoryNamespaces)

		targets, err := r.selectedClusterTargets(ctx, &inv, inventoryNamespaces)
		if err != nil {
			retErr = err
			return ctrl.Result{}, err
		}

		if len(targets) == 0 {
			if inv.CreationTimestamp.Time.Add(clusterTargetBootstrapWindow).After(time.Now()) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			return r.setDegraded(ctx, &inv, "NoTargets", "no KollectClusterTarget objects matched")
		}

		itemCount, degradedTargets := r.rollupCounts(&inv, targets, selectedNamespaceNames)
		if len(degradedTargets) > 0 {
			msg := fmt.Sprintf("%d target(s) not Ready: %v", len(degradedTargets), degradedTargets)
			return r.setDegraded(ctx, &inv, "TargetDegraded", msg)
		}

		sinkNS := inv.Spec.SinkNamespace
		if sinkNS == "" {
			sinkNS = sink.DefaultSecretNamespace
		}

		bindings := clusterInventorySinkBindings(&inv)
		scopeResult, enforceErr := r.enforceClusterScopePolicy(ctx, &inv, sinkNS, bindings)
		if enforceErr != nil {
			retErr = enforceErr
			return ctrl.Result{}, enforceErr
		}
		if !scopeResult.IsZero() {
			return scopeResult, nil
		}

		if len(bindings) > 0 {
			sinkOK, sinkReason, sinkMsg := checkClusterInventorySinksReachable(ctx, r.Client, sinkNS, bindings)
			setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, sinkOK, sinkReason, sinkMsg)
			if !sinkOK {
				recordWarning(r.Recorder, &inv, sinkReason, sinkMsg)
				return r.setDegraded(ctx, &inv, sinkReason, sinkMsg)
			}
		} else {
			setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, true, "NoSinksConfigured", "no family sink refs configured")
		}

		if r.Store == nil || r.Engine == nil {
			return r.updateStatus(ctx, &inv, len(targets), itemCount, perSinkExportOutcome{RequeueAfter: r.exportDebounce(&inv)}, nil)
		}

		result, err := r.reconcileRollupExport(ctx, req, &inv, targets, selectedNamespaceNames, sinkNS, log)
		if err != nil {
			retErr = err
		}

		return result, err
	})
}

//nolint:logcheck // cluster rollup export passes named reconcile logger alongside ctx deadline
func (r *KollectClusterInventoryReconciler) reconcileRollupExport(
	ctx context.Context,
	req ctrl.Request,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targets []kollectdevv1alpha1.KollectClusterTarget,
	selectedNamespaceNames map[string]struct{},
	sinkNS string,
	log logr.Logger,
) (ctrl.Result, error) {
	bindings := clusterInventorySinkBindings(inv)
	rollup, err := r.composeNamespaceRollup(inv, targets, selectedNamespaceNames)
	if err != nil {
		return ctrl.Result{}, err
	}

	gate, err := assessExportSpill(
		ctx, r.Client, log, int64(len(rollup.Payload)), validation.MaxExportBytesGlobal(), sinkNS, bindings,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if gate.degraded {
		recordSpillGateMetrics(gate)

		return r.setDegraded(ctx, inv, gate.reason, gate.message)
	}

	key := req.String()
	itemCount := len(rollup.Items)

	bindings = clusterInventorySinkBindings(inv)
	if len(bindings) == 0 {
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no family sink refs configured")
		return r.updateStatus(ctx, inv, len(targets), itemCount, perSinkExportOutcome{RequeueAfter: r.exportDebounce(inv)}, rollup.NamespaceShards)
	}

	if r.Registry == nil {
		return r.setDegraded(ctx, inv, "ExportUnavailable", "sink registry is not configured")
	}

	outcome := r.exportClusterToSinks(ctx, log, inv, key, sinkNS, rollup.Items, rollup.Checksum)

	if isTotalExportFailure(outcome) {
		metrics.ReconcileErrorsTotal.WithLabelValues(
			"KollectClusterInventory", kollecterrors.ClassOf(outcome.ExportErr),
		).Inc()
		reason := reasonProgressing
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			reason = kollectdevv1alpha1.ReasonExportTerminal
		}
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, outcome.ExportErr)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, outcome.ExportErr.Error())
		recordWarning(r.Recorder, inv, reason, outcome.ExportErr.Error())

		result, err := r.setDegraded(ctx, inv, reason, outcome.ExportErr.Error())
		if kollecterrors.IsTerminal(outcome.ExportErr) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	if outcome.ExportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues(
			"KollectClusterInventory", kollecterrors.ClassOf(outcome.ExportErr),
		).Inc()
		recordWarning(r.Recorder, inv, reasonExportFailed, outcome.ExportErr.Error())
	}

	return r.updateStatus(ctx, inv, len(targets), itemCount, outcome, rollup.NamespaceShards)
}

//nolint:logcheck // per-sink export passes named reconcile logger alongside ctx deadline
func (r *KollectClusterInventoryReconciler) exportClusterToSinks(
	ctx context.Context,
	log logr.Logger,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	invKey, sinkNS string,
	items []collect.Item,
	checksum string,
) perSinkExportOutcome {
	now := time.Now()
	defaultInterval := r.exportDebounce(inv)
	scopeFloor := r.clusterScopeFloor(ctx, sinkNS)

	var outcome perSinkExportOutcome
	outcome.RequeueAfter = defaultInterval

	for _, binding := range clusterInventorySinkBindings(inv) {
		ref := binding.Ref
		exportKey := sinkExportKey(binding)
		resolved, err := loadClusterInventorySink(ctx, r.Client, sinkNS, binding)
		status := upsertSinkExportStatus(&outcome.SinkExports, exportKey)
		status.Namespace = sinkBindingNamespace(binding, sinkNS)
		if err != nil {
			setSinkExportSynced(status, inv.Generation, false, reasonExportFailed, err.Error())
			outcome.addSinkFailure(exportKey, err)
			continue
		}

		interval := validation.ResolveSinkExportInterval(ref, sinkExportMinInterval(resolved), defaultInterval, scopeFloor)

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

		if err := sink.RunExportItems(sink.ExportItemsRequest{
			Ctx:           ctx,
			Client:        r.Client,
			Registry:      r.Registry,
			SinkNamespace: sink.SinkNamespaceForResolved(resolved, sinkNS),
			SinkName:      binding.Name,
			SinkFamily:    binding.Family,
			ObjectPath:    fmt.Sprintf("inventory/cluster/%s.json", inv.Name),
			Items:         items,
			Meta:          export.Metadata{Generation: inv.Generation},
		}); err != nil {
			log.Error(err, "cluster export failed", "sink", exportKey)
			outcome.addSinkFailure(exportKey, err)
			setSinkExportSynced(status, inv.Generation, false, reasonExportFailed, err.Error())
			continue
		}

		r.sinkCoalesce.record(invKey, exportKey, inv.Generation, checksum, now)
		exportTime := metav1.Now()
		status.LastExportTime = &exportTime
		status.LastChecksum = checksum
		setSinkExportSynced(status, inv.Generation, true, "Exported", "export completed")
		outcome.ExportedCount++
		outcome.RequeueAfter = mergeRequeueAfter(outcome.RequeueAfter, validation.RequeueAfterForZeroInterval(interval))
	}

	return outcome
}

func (r *KollectClusterInventoryReconciler) clusterScopeFloor(ctx context.Context, sinkNS string) time.Duration {
	binding, err := scope.Load(ctx, r.Client, sinkNS)
	if err != nil || !binding.Enforced || binding.Scope == nil {
		return 0
	}
	return validation.ScopeMinExportInterval(binding.Scope)
}

func (r *KollectClusterInventoryReconciler) exportDebounce(
	inv *kollectdevv1alpha1.KollectClusterInventory,
) time.Duration {
	return validation.ClusterExportMinIntervalFor(&inv.Spec, 0)
}

func (r *KollectClusterInventoryReconciler) collectRollupItems(
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targets []kollectdevv1alpha1.KollectClusterTarget,
	selectedNamespaceNames map[string]struct{},
) []collect.Item {
	var items []collect.Item
	for i := range targets {
		ct := &targets[i]
		for _, ns := range r.Engine.NamespacesForClusterTarget(ct.Name) {
			if len(selectedNamespaceNames) > 0 {
				if _, ok := selectedNamespaceNames[ns]; !ok {
					continue
				}
			}
			items = append(items, r.Store.SnapshotTarget(ns, ct.Name)...)
		}
	}

	return aggregate.MergeRows(items, aggregate.DedupeModeFromSpec(&inv.Spec))
}

type clusterRollup struct {
	Items           []collect.Item
	Payload         []byte
	Checksum        string
	NamespaceShards []kollectdevv1alpha1.InventoryNamespaceShardStatus
}

type namespaceRollupShard struct {
	namespace string
	items     []collect.Item
	targets   map[string]struct{}
}

func (r *KollectClusterInventoryReconciler) composeNamespaceRollup(
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targets []kollectdevv1alpha1.KollectClusterTarget,
	selectedNamespaceNames map[string]struct{},
) (clusterRollup, error) {
	shards := make(map[string]*namespaceRollupShard)
	for i := range targets {
		ct := &targets[i]
		for _, ns := range r.Engine.NamespacesForClusterTarget(ct.Name) {
			if len(selectedNamespaceNames) > 0 {
				if _, ok := selectedNamespaceNames[ns]; !ok {
					continue
				}
			}

			snapshot := r.Store.SnapshotTarget(ns, ct.Name)
			if len(snapshot) == 0 {
				continue
			}

			shard, ok := shards[ns]
			if !ok {
				shard = &namespaceRollupShard{
					namespace: ns,
					targets:   make(map[string]struct{}),
				}
				shards[ns] = shard
			}
			shard.items = append(shard.items, snapshot...)
			shard.targets[ct.Name] = struct{}{}
		}
	}

	namespaceNames := make([]string, 0, len(shards))
	for ns := range shards {
		namespaceNames = append(namespaceNames, ns)
	}
	slices.Sort(namespaceNames)

	dedupeMode := aggregate.DedupeModeFromSpec(&inv.Spec)
	namespaceShards := make([]kollectdevv1alpha1.InventoryNamespaceShardStatus, 0, len(namespaceNames))
	composed := make([]collect.Item, 0)
	namespaceChecksums := make([]string, 0, len(namespaceNames))

	for _, namespace := range namespaceNames {
		shard := shards[namespace]
		shardItems := aggregate.MergeRows(shard.items, dedupeMode)
		checksum, err := export.ItemsFingerprint(shardItems)
		if err != nil {
			return clusterRollup{}, err
		}

		namespaceShards = append(namespaceShards, kollectdevv1alpha1.InventoryNamespaceShardStatus{
			Namespace:   namespace,
			ItemCount:   len(shardItems),
			TargetCount: len(shard.targets),
			Checksum:    checksum,
		})
		namespaceChecksums = append(namespaceChecksums, namespace+":"+checksum)
		composed = append(composed, shardItems...)
	}

	items := aggregate.MergeRows(composed, dedupeMode)
	payload, err := collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: inv.Generation,
	})
	if err != nil {
		return clusterRollup{}, err
	}

	return clusterRollup{
		Items:           items,
		Payload:         payload,
		Checksum:        composeNamespaceShardChecksum(namespaceChecksums),
		NamespaceShards: namespaceShards,
	}, nil
}

func composeNamespaceShardChecksum(shardChecksums []string) string {
	h := sha256.New()
	for i := range shardChecksums {
		_, _ = h.Write([]byte(shardChecksums[i]))
		_, _ = h.Write([]byte{'\n'})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (r *KollectClusterInventoryReconciler) selectedClusterTargets(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	inventoryNamespaces []corev1.Namespace,
) ([]kollectdevv1alpha1.KollectClusterTarget, error) {
	var list kollectdevv1alpha1.KollectClusterTargetList
	if err := r.List(ctx, &list); err != nil {
		return nil, err
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

		if !clusterTargetMatchesInventoryNS(&ct, inventoryNamespaces) {
			continue
		}

		selected = append(selected, ct)
	}

	return selected, nil
}

func targetSelectorFor(inv *kollectdevv1alpha1.KollectClusterInventory) (labels.Selector, error) {
	if len(inv.Spec.TargetRefs) > 0 || inv.Spec.TargetSelector == nil {
		return nil, nil
	}

	return metav1.LabelSelectorAsSelector(inv.Spec.TargetSelector)
}

func (r *KollectClusterInventoryReconciler) selectedInventoryNamespaces(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) ([]corev1.Namespace, error) {
	invSel, err := metav1.LabelSelectorAsSelector(inv.Spec.NamespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("namespaceSelector: %w", err)
	}

	explicitNames := make(map[string]struct{}, len(inv.Spec.Namespaces))
	for _, name := range inv.Spec.Namespaces {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		explicitNames[trimmed] = struct{}{}
	}

	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList); err != nil {
		return nil, err
	}

	filtered := make([]corev1.Namespace, 0, len(nsList.Items))
	for i := range nsList.Items {
		ns := nsList.Items[i]
		if len(explicitNames) > 0 {
			if _, ok := explicitNames[ns.Name]; !ok {
				continue
			}
		}

		if !invSel.Matches(labels.Set(ns.Labels)) {
			continue
		}

		filtered = append(filtered, ns)
	}

	return filtered, nil
}

func namespaceNameSet(namespaces []corev1.Namespace) map[string]struct{} {
	set := make(map[string]struct{}, len(namespaces))
	for i := range namespaces {
		set[namespaces[i].Name] = struct{}{}
	}

	return set
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
	ct *kollectdevv1alpha1.KollectClusterTarget,
	inventoryNamespaces []corev1.Namespace,
) bool {
	targetSel, err := metav1.LabelSelectorAsSelector(ct.Spec.NamespaceSelector)
	if err != nil {
		return false
	}

	for i := range inventoryNamespaces {
		ns := &inventoryNamespaces[i]
		nsLabels := labels.Set(ns.Labels)
		if targetSel.Matches(nsLabels) {
			return true
		}
	}

	return false
}

func (r *KollectClusterInventoryReconciler) rollupCounts(
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targets []kollectdevv1alpha1.KollectClusterTarget,
	selectedNamespaceNames map[string]struct{},
) (itemCount int, degraded []string) {
	for i := range targets {
		ct := &targets[i]
		if !clusterTargetReady(ct) {
			degraded = append(degraded, ct.Name)
		}
	}

	if r.Engine != nil && r.Store != nil {
		return len(r.collectRollupItems(inv, targets, selectedNamespaceNames)), degraded
	}

	for i := range targets {
		ct := &targets[i]
		if r.Engine != nil {
			for _, ns := range r.Engine.NamespacesForClusterTarget(ct.Name) {
				if len(selectedNamespaceNames) > 0 {
					if _, ok := selectedNamespaceNames[ns]; !ok {
						continue
					}
				}
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
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) (bool, string, string) {
	return checkInventorySinksReachable(ctx, c, sinkNamespace, bindings)
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
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, nil
}

func (r *KollectClusterInventoryReconciler) updateStatus(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targetCount, itemCount int,
	outcome perSinkExportOutcome,
	namespaceShards []kollectdevv1alpha1.InventoryNamespaceShardStatus,
) (ctrl.Result, error) {
	inv.Status.ObservedGeneration = inv.Generation
	inv.Status.TargetCount = targetCount
	inv.Status.ItemCount = itemCount
	inv.Status.NamespaceShardCount = len(namespaceShards)
	inv.Status.NamespaceShards = namespaceShards
	inv.Status.SinkExports = outcome.SinkExports

	bindings := clusterInventorySinkBindings(inv)
	if len(bindings) > 0 {
		if latest := latestExportTime(outcome.SinkExports); latest != nil {
			inv.Status.LastExportTime = latest
		}

		failed := outcome.FailedCount
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, outcome.ExportErr)
		aggregateInventorySync(&inv.Status.Conditions, inv.Generation,
			outcome.ExportedCount, outcome.DebouncedCount, failed)

		switch {
		case failed == 0 && outcome.ExportErr == nil:
			apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
			if outcome.ExportedCount > 0 {
				recordNormal(r.Recorder, inv, "ExportSucceeded",
					fmt.Sprintf("exported %d item(s) from %d target(s) to %d sink(s)",
						itemCount, targetCount, outcome.ExportedCount))
			}
			apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
				Type:               kollectdevv1alpha1.ConditionExportSucceeded,
				Status:             metav1.ConditionTrue,
				Reason:             "Exported",
				Message:            fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, outcome.ExportedCount),
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
		case outcome.ExportedCount > 0:
			apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
			apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
				Type:               kollectdevv1alpha1.ConditionExportSucceeded,
				Status:             metav1.ConditionTrue,
				Reason:             kollectdevv1alpha1.ReasonPartiallySynced,
				Message:            fmt.Sprintf("exported %d item(s) to %d/%d sink(s)", itemCount, outcome.ExportedCount, totalClusterInventorySinkRefs(inv)),
				ObservedGeneration: inv.Generation,
				LastTransitionTime: metav1.Now(),
			})
			apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             kollectdevv1alpha1.ReasonPartiallySynced,
				Message:            fmt.Sprintf("rolled up %d target(s), %d item(s)", targetCount, itemCount),
				ObservedGeneration: inv.Generation,
				LastTransitionTime: metav1.Now(),
			})
		}
	} else if outcome.ExportErr == nil {
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

	requeue := outcome.RequeueAfter
	if requeue <= 0 {
		requeue = r.exportDebounce(inv)
	}

	return ctrl.Result{RequeueAfter: requeue}, nil
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
			&kollectdevv1alpha1.KollectSnapshotSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterSnapshotSinkToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectDatabaseSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterDatabaseSinkToInventories),
		).
		Watches(
			&kollectdevv1alpha1.KollectEventSink{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterEventSinkToInventories),
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
		logf.FromContext(ctx).Error(err, "failed to list cluster inventories for cluster target watch mapping")

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

func (r *KollectClusterInventoryReconciler) mapClusterSnapshotSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapClusterFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilySnapshot)
}

func (r *KollectClusterInventoryReconciler) mapClusterDatabaseSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapClusterFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilyDatabase)
}

func (r *KollectClusterInventoryReconciler) mapClusterEventSinkToInventories(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapClusterFamilySinkToInventories(ctx, obj, kollectdevv1alpha1.SinkFamilyEvent)
}

func (r *KollectClusterInventoryReconciler) mapClusterFamilySinkToInventories(
	ctx context.Context,
	obj client.Object,
	family string,
) []reconcile.Request {
	sinkName := obj.GetName()
	sinkNS := obj.GetNamespace()

	var list kollectdevv1alpha1.KollectClusterInventoryList
	if err := r.List(ctx, &list); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list cluster inventories for sink watch mapping",
			"sink", sinkName, "namespace", sinkNS, "family", family)

		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		inv := &list.Items[i]
		invSinkNS := inv.Spec.SinkNamespace
		if invSinkNS == "" {
			invSinkNS = sink.DefaultSecretNamespace
		}
		for _, binding := range clusterInventorySinkBindings(inv) {
			if binding.Family != family || binding.Name != sinkName {
				continue
			}
			if sinkNS != "" && sinkBindingNamespace(binding, invSinkNS) != sinkNS {
				continue
			}
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: inv.Name}})
			break
		}
	}

	return reqs
}
