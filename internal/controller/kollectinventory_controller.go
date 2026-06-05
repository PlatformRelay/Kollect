// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/spoke"
)

// KollectInventoryReconciler reconciles a KollectInventory object
type KollectInventoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    *collect.Store
	Registry *sink.Registry
	Options  RuntimeOptions

	mu          sync.Mutex
	lastExport  map[string]time.Time
	lastPayload map[string]string
}

func (r *KollectInventoryReconciler) exportDebounce() time.Duration {
	if r.Options.ExportDebounce > 0 {
		return r.Options.ExportDebounce
	}

	return DefaultRuntimeOptions().ExportDebounce
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectinventories/finalizers,verbs=update
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecttargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

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

	if r.Store == nil {
		return ctrl.Result{}, nil
	}

	payload, err := r.Store.MarshalNamespaceJSON(inv.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	hash := payloadHash(payload)
	key := req.String()

	if r.shouldDebounce(key, hash) {
		debounce := r.exportDebounce()
		delay := debounce - time.Since(r.lastExportTime(key))
		if delay < time.Second {
			delay = time.Second
		}

		return ctrl.Result{RequeueAfter: delay}, nil
	}

	itemCount := r.Store.CountForNamespace(inv.Namespace)

	if err := spoke.TryPublishReport(ctx, r.Store, &inv); err != nil {
		log.Error(err, "spoke hub publish")
	}

	if len(inv.Spec.SinkRefs) == 0 {
		return r.updateStatus(ctx, &inv, itemCount, nil)
	}

	var exportErr error
	for _, sinkName := range inv.Spec.SinkRefs {
		if err := r.exportToSink(ctx, &inv, sinkName, payload); err != nil {
			log.Error(err, "export failed", "sink", sinkName)
			exportErr = err
		}
	}

	r.recordExport(key, hash)

	if exportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectInventory", metrics.ErrorClassTransient).Inc()

		return r.setInventoryDegraded(ctx, &inv, itemCount, exportErr)
	}

	return r.updateStatus(ctx, &inv, itemCount, nil)
}

func (r *KollectInventoryReconciler) exportToSink(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	sinkName string,
	payload []byte,
) error {
	var ks kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, client.ObjectKey{Name: sinkName}, &ks); err != nil {
		return fmt.Errorf("load KollectSink %q: %w", sinkName, err)
	}

	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, ks.Spec, inv.Namespace)
	if err != nil {
		return err
	}

	backend, err := r.Registry.NewBackend(ks.Spec, buildCtx)
	if err != nil {
		return err
	}

	objectPath := fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name)

	start := time.Now()
	err = backend.Export(ctx, payload, objectPath)
	elapsed := time.Since(start).Seconds()
	metrics.ExportDurationSeconds.WithLabelValues(ks.Spec.Type).Observe(elapsed)
	metrics.ExportBytesTotal.WithLabelValues(ks.Spec.Type).Add(float64(len(payload)))

	return err
}

func (r *KollectInventoryReconciler) shouldDebounce(key, hash string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastPayload == nil {
		r.lastPayload = make(map[string]string)
		r.lastExport = make(map[string]time.Time)
	}

	prev, ok := r.lastPayload[key]
	if !ok || prev != hash {
		return false
	}

	return time.Since(r.lastExport[key]) < r.exportDebounce()
}

func (r *KollectInventoryReconciler) recordExport(key, hash string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastPayload == nil {
		r.lastPayload = make(map[string]string)
		r.lastExport = make(map[string]time.Time)
	}

	r.lastPayload[key] = hash
	r.lastExport[key] = time.Now()
}

func (r *KollectInventoryReconciler) lastExportTime(key string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lastExport[key]
}

func payloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}

func (r *KollectInventoryReconciler) updateStatus(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	exportErr error,
) (ctrl.Result, error) {
	inv.Status.ObservedGeneration = inv.Generation
	inv.Status.ItemCount = itemCount

	if exportErr == nil {
		now := metav1.Now()
		inv.Status.LastExportTime = &now
		apimeta.RemoveStatusCondition(&inv.Status.Conditions, conditionDegraded)
		apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Exported",
			Message:            fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, len(inv.Spec.SinkRefs)),
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

	return ctrl.Result{RequeueAfter: r.exportDebounce()}, nil
}

func (r *KollectInventoryReconciler) setInventoryDegraded(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	exportErr error,
) (ctrl.Result, error) {
	inv.Status.ItemCount = itemCount
	apimeta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             "ExportFailed",
		Message:            exportErr.Error(),
		ObservedGeneration: inv.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, inv); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.exportDebounce()}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentInventory)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentInventory
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectInventory{}).
		WithOptions(opts).
		Watches(
			&kollectdevv1alpha1.KollectTarget{},
			handler.EnqueueRequestsFromMapFunc(r.mapTargetToInventories),
		).
		Named("kollectinventory").
		Complete(r)
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
