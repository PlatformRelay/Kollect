// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

type relationalRecordingBackend struct {
	exported [][]byte
}

func (r *relationalRecordingBackend) Type() string { return "relational-recording" }

func (r *relationalRecordingBackend) Capabilities() sink.Capabilities {
	return sink.RelationalStoreCapabilities()
}

func (r *relationalRecordingBackend) Export(_ context.Context, payload []byte, _ string) error {
	r.exported = append(r.exported, append([]byte(nil), payload...))

	return nil
}

func TestKollectInventoryReconciler_addsCleanupFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	rec := &KollectInventoryReconciler{
		Client: cl,
		Scheme: scheme,
		Store:  collect.NewStore(),
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if !containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
		t.Fatalf("finalizers = %v, want %q", got.Finalizers, inventoryCleanupFinalizer)
	}
}

func TestKollectInventoryReconciler_deleteExportsEmptyAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "web",
		UID:             "uid-1",
		Namespace:       "default",
		Name:            "demo",
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

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
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

	recorder := &relationalRecordingBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
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
		t.Fatalf("cleanup export count = %d, want 1", len(recorder.exported))
	}
	if !strings.Contains(string(recorder.exported[0]), `"items":[]`) {
		t.Fatalf("cleanup payload = %s, want empty items envelope", recorder.exported[0])
	}

	var got kollectdevv1alpha1.KollectInventory
	err = cl.Get(context.Background(), types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got)
	if err == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after cleanup: %v", got.Finalizers)
		}
	}
}
