// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

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
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

// KollectClusterInventoryReconciler rolls up status from KollectClusterTarget objects.
// Export to sinks is stubbed until cluster export wiring lands.
type KollectClusterInventoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    *collect.Store
	Engine   *collect.Engine
	Options  RuntimeOptions
	Recorder record.EventRecorder
}

//nolint:lll // kubebuilder rbac marker length
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterinventories/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustertargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile aggregates cluster target status into rollup conditions (export stub).
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
		if !sinkOK {
			recordWarning(r.Recorder, &inv, sinkReason, sinkMsg)
			return r.setDegraded(ctx, &inv, sinkReason, sinkMsg)
		}

		log.Info("cluster inventory export deferred", "inventory", inv.Name, "sinks", len(inv.Spec.SinkRefs))
	}

	return r.setReady(ctx, &inv, len(targets), itemCount)
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

	return ctrl.Result{RequeueAfter: defaultCollectRequeue}, nil
}

func (r *KollectClusterInventoryReconciler) setReady(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	targetCount, itemCount int,
) (ctrl.Result, error) {
	apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
	inv.Status.ObservedGeneration = inv.Generation
	inv.Status.TargetCount = targetCount
	inv.Status.ItemCount = itemCount

	msg := fmt.Sprintf("rolled up %d target(s), %d item(s); export stub", targetCount, itemCount)
	setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "RolledUp", msg)
	apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "RolledUp",
		Message:            msg,
		ObservedGeneration: inv.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if len(inv.Spec.SinkRefs) > 0 {
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionExportSucceeded,
			Status:             metav1.ConditionFalse,
			Reason:             "ExportDeferred",
			Message:            "cluster inventory export wiring is not implemented yet",
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

	return ctrl.Result{RequeueAfter: defaultCollectRequeue}, nil
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
