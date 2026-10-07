// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectdatabasesinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectdatabasesinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecteventsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecteventsinks/status,verbs=get;update;patch

// familySinkPtr constrains a type parameter T to a family-sink CRD kind
// accessed through its pointer receiver: a client.Object that also satisfies
// FamilySinkObject. This lets FamilySinkReconciler drive KollectSnapshotSink,
// KollectDatabaseSink, and KollectEventSink through one implementation
// (AR-08) instead of three near-identical reconcilers.
type familySinkPtr[T any] interface {
	*T
	client.Object
	kollectdevv1alpha1.FamilySinkObject
}

// FamilySinkReconciler runs connection tests for any family-sink CRD kind
// (KollectSnapshotSink, KollectDatabaseSink, KollectEventSink).
type FamilySinkReconciler[T any, PT familySinkPtr[T]] struct {
	client.Client
	Scheme *runtime.Scheme
	// Name is both the controller name (SetupWithManager) and the metric
	// label used by trackReconcile — the three concrete kinds always used
	// the same string for both, so one field covers it.
	Name string
}

func (r *FamilySinkReconciler[T, PT]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile(r.Name)
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var obj T
	p := PT(&obj)
	if err := r.Get(ctx, req.NamespacedName, p); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return guardReconcile(ctx, nil, p, func() (ctrl.Result, error) {
		conn := familySinkConnection{client: r.Client}
		status := p.FamilySinkStatus()
		err := conn.reconcile(ctx, p, p.FamilySinkSpec(), p.FamilySinkCommon(), &status.Conditions, &status.Preview)
		if err != nil {
			log.Error(err, "sink connection test failed", "controller", r.Name)
			retErr = err
		}

		return ctrl.Result{}, err
	})
}

// evictBackendPoolOnSinkDelete is the DeleteFunc body of the family-sink
// delete watches (BEP-1): evict the deleted sink's pooled backend through the
// pool's delete-hook seam. The handler value only implements Delete — Create,
// Update and Generic stay no-ops, so live sinks are never evicted by this
// watch (their spec changes swap the pooled entry on acquire). controller-
// runtime unwraps DeletedFinalStateUnknown tombstones before DeleteFunc runs
// (internal/source/event_handler.go OnDelete), so the object carries the
// sink's last-known identity; the seam falls back to namespace/name only when
// that object carries no UID.
func evictBackendPoolOnSinkDelete(obj client.Object) {
	if obj == nil {
		return
	}

	sink.EvictBackendPoolForSink(obj.GetUID(), obj.GetNamespace(), obj.GetName())
}

func (r *FamilySinkReconciler[T, PT]) SetupWithManager(mgr ctrl.Manager) error {
	var t T

	// BEP-1: a delete watch per kind evicts the deleted sink's pooled backend
	// immediately instead of holding it for the idle TTL. The watch enqueues
	// nothing: For() still delivers every event for reconcile, and eviction
	// must not re-export, retract or reconcile anything.
	return ctrl.NewControllerManagedBy(mgr).
		For(PT(&t)).
		Watches(
			PT(&t),
			handler.Funcs{
				DeleteFunc: func(_ context.Context, e event.DeleteEvent, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
					evictBackendPoolOnSinkDelete(e.Object)
				},
			},
		).
		Named(r.Name).
		Complete(r)
}
