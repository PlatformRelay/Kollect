// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

type recordingBackend struct {
	mu       sync.Mutex
	exported [][]byte
}

func (r *recordingBackend) Type() string { return "recording" }

func (r *recordingBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (r *recordingBackend) Export(_ context.Context, payload []byte, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exported = append(r.exported, append([]byte(nil), payload...))

	return nil
}

func TestKollectInventoryReconciler_exportsDeploymentToSink(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "nginx-deployments",
		UID:             "uid-nginx",
		Namespace:       "default",
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
		Attributes:      map[string]any{"image": "nginx:1.27-alpine"},
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(recorder.exported) != 1 {
		t.Fatalf("export count = %d, want 1", len(recorder.exported))
	}

	var got kollectdevv1alpha1.KollectInventory
	invKey := types.NamespacedName{Name: "team-inventory", Namespace: "default"}
	if err := cl.Get(context.Background(), invKey, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if got.Status.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", got.Status.ItemCount)
	}

	if len(got.Status.SinkExports) != 1 {
		t.Fatalf("SinkExports = %d, want 1", len(got.Status.SinkExports))
	}
	wantName := string(kollectdevv1alpha1.SinkFamilyDatabase) + "/postgres-demo"
	if got.Status.SinkExports[0].Name != wantName {
		t.Fatalf("sink export name = %q, want %s", got.Status.SinkExports[0].Name, wantName)
	}
	if got.Status.SinkExports[0].LastExportTime == nil {
		t.Fatal("expected lastExportTime on sinkExports[0]")
	}
}

// newExportFailureReconciler builds a reconciler whose single postgres sink
// always fails export with exportErr.
func newExportFailureReconciler(t *testing.T, exportErr error) (*KollectInventoryReconciler, client.Client) {
	t.Helper()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "nginx-deployments",
		UID:             "uid-nginx",
		Namespace:       "default",
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return &failingRelationalBackend{err: exportErr}, nil
	})

	return &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}, cl
}

// WB-01: a terminal total export failure must not requeue — empty result,
// nil error, Degraded with reason ReasonExportTerminal.
func TestKollectInventoryReconciler_terminalTotalExportFailureDoesNotRequeue(t *testing.T) {
	t.Parallel()

	rec, cl := newExportFailureReconciler(t,
		kollecterrors.Terminal(errors.New("bucket does not exist")))

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil (terminal export must not requeue)", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("RequeueAfter = %v, want 0 (terminal export must not requeue)", result.RequeueAfter)
	}
	if result.Requeue { //nolint:staticcheck // SA1019: asserting Requeue stays unset
		t.Fatal("Requeue = true, want false (terminal export must not requeue)")
	}

	var got kollectdevv1alpha1.KollectInventory
	invKey := types.NamespacedName{Name: "team-inventory", Namespace: "default"}
	if getErr := cl.Get(context.Background(), invKey, &got); getErr != nil {
		t.Fatalf("Get inventory: %v", getErr)
	}

	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue ||
		degraded.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("Degraded condition = %+v, want True with reason %q",
			degraded, kollectdevv1alpha1.ReasonExportTerminal)
	}
}

// WB-01 counterpart: a transient total export failure keeps requeueing.
func TestKollectInventoryReconciler_transientTotalExportFailureRequeues(t *testing.T) {
	t.Parallel()

	rec, _ := newExportFailureReconciler(t,
		kollecterrors.Transient(errors.New("connection refused")))

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil (status-driven retry)", err)
	}
	if result.RequeueAfter <= 0 {
		t.Fatalf("RequeueAfter = %v, want > 0 (transient export must requeue)", result.RequeueAfter)
	}
}
