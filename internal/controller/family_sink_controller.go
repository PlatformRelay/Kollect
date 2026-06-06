// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// FamilySnapshotSinkReconciler runs connection tests for KollectSnapshotSink.
type FamilySnapshotSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustersnapshotsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustersnapshotsinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *FamilySnapshotSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectsnapshotsink")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var obj kollectdevv1alpha1.KollectSnapshotSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	if err != nil {
		log.Error(err, "snapshot sink connection test failed")
		retErr = err
	}

	return ctrl.Result{}, err
}

func (r *FamilySnapshotSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectSnapshotSink{}).
		Named("kollectsnapshotsink").
		Complete(r)
}

// FamilyClusterSnapshotSinkReconciler runs connection tests for KollectClusterSnapshotSink.
type FamilyClusterSnapshotSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FamilyClusterSnapshotSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectclustersnapshotsink")
	var retErr error
	defer func() { finish(retErr) }()

	var obj kollectdevv1alpha1.KollectClusterSnapshotSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	retErr = err

	return ctrl.Result{}, err
}

func (r *FamilyClusterSnapshotSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectClusterSnapshotSink{}).
		Named("kollectclustersnapshotsink").
		Complete(r)
}

// FamilyDatabaseSinkReconciler runs connection tests for KollectDatabaseSink.
type FamilyDatabaseSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectdatabasesinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectdatabasesinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterdatabasesinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclusterdatabasesinks/status,verbs=get;update;patch

func (r *FamilyDatabaseSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectdatabasesink")
	var retErr error
	defer func() { finish(retErr) }()

	var obj kollectdevv1alpha1.KollectDatabaseSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	retErr = err

	return ctrl.Result{}, err
}

func (r *FamilyDatabaseSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectDatabaseSink{}).
		Named("kollectdatabasesink").
		Complete(r)
}

// FamilyClusterDatabaseSinkReconciler runs connection tests for KollectClusterDatabaseSink.
type FamilyClusterDatabaseSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FamilyClusterDatabaseSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectclusterdatabasesink")
	var retErr error
	defer func() { finish(retErr) }()

	var obj kollectdevv1alpha1.KollectClusterDatabaseSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	retErr = err

	return ctrl.Result{}, err
}

func (r *FamilyClusterDatabaseSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectClusterDatabaseSink{}).
		Named("kollectclusterdatabasesink").
		Complete(r)
}

// FamilyEventSinkReconciler runs connection tests for KollectEventSink.
type FamilyEventSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollecteventsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecteventsinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustereventsinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollectclustereventsinks/status,verbs=get;update;patch

func (r *FamilyEventSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollecteventsink")
	var retErr error
	defer func() { finish(retErr) }()

	var obj kollectdevv1alpha1.KollectEventSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	retErr = err

	return ctrl.Result{}, err
}

func (r *FamilyEventSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectEventSink{}).
		Named("kollecteventsink").
		Complete(r)
}

// FamilyClusterEventSinkReconciler runs connection tests for KollectClusterEventSink.
type FamilyClusterEventSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FamilyClusterEventSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollectclustereventsink")
	var retErr error
	defer func() { finish(retErr) }()

	var obj kollectdevv1alpha1.KollectClusterEventSink
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conn := familySinkConnection{client: r.Client}
	err := conn.reconcile(ctx, &obj, obj.Spec.ToKollectSinkSpec(), &obj.Spec.SinkCommonFields, &obj.Status.Conditions)
	retErr = err

	return ctrl.Result{}, err
}

func (r *FamilyClusterEventSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectClusterEventSink{}).
		Named("kollectclustereventsink").
		Complete(r)
}
