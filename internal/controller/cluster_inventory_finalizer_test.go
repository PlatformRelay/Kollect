// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

func TestKollectClusterInventoryReconciler_deleteExportsEmptyAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform-rollup",
			Finalizers:        []string{clusterInventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &relationalRecordingBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform-rollup"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(recorder.exported) != 1 {
		t.Fatalf("cleanup export count = %d, want 1", len(recorder.exported))
	}
	if !strings.Contains(string(recorder.exported[0]), `"items":[]`) {
		t.Fatalf("cleanup payload = %s, want empty items envelope", recorder.exported[0])
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	err = cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got)
	if err == nil && containsFinalizer(got.Finalizers, clusterInventoryCleanupFinalizer) {
		t.Fatalf("finalizer still present after cleanup: %v", got.Finalizers)
	}
}

// EC-P1-03: a terminal cleanup failure must not requeue (nil error, empty
// result) and must keep the finalizer in place for manual intervention.
func TestKollectClusterInventoryReconciler_terminalCleanupDoesNotRequeue(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform-rollup",
			Finalizers:        []string{clusterInventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &failingRelationalBackend{
			err: kollecterrors.Terminal(errors.New("table schema is invalid")),
		}, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform-rollup"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil (terminal cleanup must not requeue)", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("Reconcile result = %+v, want empty result (no requeue)", result)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if getErr := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); getErr != nil {
		t.Fatalf("Get cluster inventory: %v", getErr)
	}
	if !containsFinalizer(got.Finalizers, clusterInventoryCleanupFinalizer) {
		t.Fatalf("finalizer removed despite failed cleanup: %v", got.Finalizers)
	}

	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonCleanupTerminal {
		t.Fatalf("Degraded condition = %+v, want True with reason %q", degraded, reasonCleanupTerminal)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupTerminal) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupTerminal)
		}
	default:
		t.Fatalf("expected %s warning event", reasonCleanupTerminal)
	}
}

// EC-P1-03 counterpart: transient cleanup failures keep the error-driven retry.
func TestKollectClusterInventoryReconciler_transientCleanupStillRetries(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform-rollup",
			Finalizers:        []string{clusterInventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &failingRelationalBackend{
			err: kollecterrors.Transient(errors.New("connection refused")),
		}, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform-rollup"},
	})
	if err == nil {
		t.Fatal("Reconcile err = nil, want transient cleanup error to drive retry")
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if getErr := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); getErr != nil {
		t.Fatalf("Get cluster inventory: %v", getErr)
	}
	if !containsFinalizer(got.Finalizers, clusterInventoryCleanupFinalizer) {
		t.Fatalf("finalizer removed despite failed cleanup: %v", got.Finalizers)
	}
}
