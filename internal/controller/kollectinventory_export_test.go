// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sync"
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

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default"},
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
	if got.Status.SinkExports[0].Name != "postgres-demo" {
		t.Fatalf("sink export name = %q, want postgres-demo", got.Status.SinkExports[0].Name)
	}
	if got.Status.SinkExports[0].LastExportTime == nil {
		t.Fatal("expected lastExportTime on sinkExports[0]")
	}
}
