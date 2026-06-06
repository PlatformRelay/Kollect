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
