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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/metrics"
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

	mu             sync.Mutex
	lastExport     map[string]time.Time
	lastPayload    map[string]string
	lastGeneration map[string]int64
}

func (r *KollectInventoryReconciler) exportDebounce(inv *kollectdevv1alpha1.KollectInventory) time.Duration {
	fallback := DefaultRuntimeOptions().ExportDebounce
	if r.Options.ExportDebounce > 0 {
		fallback = r.Options.ExportDebounce
	}

	return validation.ExportMinIntervalFor(&inv.Spec, fallback)
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

	sinkOK, sinkReason, sinkMsg := checkInventorySinksReachable(ctx, r.Client, inv.Namespace, inv.Spec.SinkRefs)
	setSinkReachableCondition(&inv.Status.Conditions, inv.Generation, sinkOK, sinkReason, sinkMsg)
	if !sinkOK {
		recordWarning(r.Recorder, &inv, sinkReason, sinkMsg)
		return r.setInventoryDegraded(ctx, &inv, itemCount, sinkReason, sinkMsg)
	}

	if r.Store == nil {
		return ctrl.Result{}, nil
	}

	payload, err := r.Store.MarshalNamespaceJSON(inv.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	if limit := r.maxExportBytes(&inv); limit > 0 && int64(len(payload)) > limit {
		msg := fmt.Sprintf("export payload %d bytes exceeds cap %d", len(payload), limit)
		metrics.SinkErrorsTotal.WithLabelValues("payload_too_large").Inc()

		return r.setInventoryDegraded(ctx, &inv, itemCount, "PayloadTooLarge", msg)
	}

	hash := payloadHash(payload)
	key := req.String()

	if r.shouldDebounce(&inv, key, hash) {
		debounce := r.exportDebounce(&inv)
		delay := debounce - time.Since(r.lastExportTime(key))
		if delay < time.Second {
			delay = time.Second
		}

		return ctrl.Result{RequeueAfter: delay}, nil
	}

	itemCount = r.Store.CountForNamespace(inv.Namespace)

	if err := spoke.TryPublishReport(ctx, r.Store, &inv); err != nil {
		log.Error(err, "spoke hub publish")
	}

	if len(inv.Spec.SinkRefs) == 0 {
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "NoExport", "no sinkRefs configured")
		return r.updateStatus(ctx, &inv, itemCount, nil)
	}

	var exportErr error
	for _, sinkName := range inv.Spec.SinkRefs {
		if err := r.exportToSink(ctx, &inv, sinkName, payload); err != nil {
			log.Error(err, "export failed", "sink", sinkName)
			exportErr = err
		}
	}

	if exportErr != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectInventory", kollecterrors.ClassOf(exportErr)).Inc()
		reason := "Progressing"
		if kollecterrors.IsTerminal(exportErr) {
			reason = "ExportTerminal"
		}
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, exportErr)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, false, reason, exportErr.Error())
		recordWarning(r.Recorder, &inv, reason, exportErr.Error())

		result, err := r.setInventoryDegraded(ctx, &inv, itemCount, reason, exportErr.Error())
		if kollecterrors.IsTerminal(exportErr) {
			result.RequeueAfter = 0
		}

		return result, err
	}

	r.recordExport(&inv, key, hash)

	return r.updateStatus(ctx, &inv, itemCount, nil)
}

func (r *KollectInventoryReconciler) exportToSink(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
	sinkName string,
	payload []byte,
) error {
	var ks kollectdevv1alpha1.KollectSink
	if err := r.Get(ctx, client.ObjectKey{Namespace: inv.Namespace, Name: sinkName}, &ks); err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load KollectSink %q: %w", sinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(sinkErrorReason(err)).Inc()

		return err
	}

	buildCtx, err := sink.BuildContextFromSpec(ctx, r.Client, ks.Spec, inv.Namespace)
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

	objectPath := fmt.Sprintf("inventory/%s/%s.json", inv.Namespace, inv.Name)

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

func sinkErrorReason(err error) string {
	if err == nil {
		return "unknown"
	}

	switch kollecterrors.ClassOf(err) {
	case kollecterrors.ClassTerminal:
		return "terminal"
	case kollecterrors.ClassForbidden:
		return "forbidden"
	default:
		return "transient"
	}
}

func (r *KollectInventoryReconciler) maxExportBytes(inv *kollectdevv1alpha1.KollectInventory) int64 {
	if inv.Spec.MaxExportBytes != nil && *inv.Spec.MaxExportBytes > 0 {
		return *inv.Spec.MaxExportBytes
	}

	return validation.MaxExportBytesGlobal()
}

func (r *KollectInventoryReconciler) shouldDebounce(
	inv *kollectdevv1alpha1.KollectInventory,
	key, hash string,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastPayload == nil {
		r.lastPayload = make(map[string]string)
		r.lastExport = make(map[string]time.Time)
		r.lastGeneration = make(map[string]int64)
	}

	if r.lastGeneration[key] != inv.Generation {
		return false
	}

	prev, ok := r.lastPayload[key]
	if !ok || prev != hash {
		return false
	}

	return time.Since(r.lastExport[key]) < r.exportDebounce(inv)
}

func (r *KollectInventoryReconciler) recordExport(
	inv *kollectdevv1alpha1.KollectInventory,
	key, hash string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastPayload == nil {
		r.lastPayload = make(map[string]string)
		r.lastExport = make(map[string]time.Time)
		r.lastGeneration = make(map[string]int64)
	}

	r.lastPayload[key] = hash
	r.lastExport[key] = time.Now()
	r.lastGeneration[key] = inv.Generation
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
		setSinkReachableFromExport(&inv.Status.Conditions, inv.Generation, nil)
		setSyncedCondition(&inv.Status.Conditions, inv.Generation, true, "Exported",
			fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, len(inv.Spec.SinkRefs)))
		recordNormal(r.Recorder, inv, "ExportSucceeded",
			fmt.Sprintf("exported %d item(s) to %d sink(s)", itemCount, len(inv.Spec.SinkRefs)))
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

	return ctrl.Result{RequeueAfter: r.exportDebounce(inv)}, nil
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
		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for i := range list.Items {
		for _, ref := range list.Items[i].Spec.SinkRefs {
			if ref == sinkObj.Name {
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
