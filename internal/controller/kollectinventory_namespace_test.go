// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestKollectInventoryReconciler_aggregatesSameNamespaceOnly(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "tenant-a",
		TargetName:      "deploys",
		UID:             "a1",
		Namespace:       "tenant-a",
		Name:            "app-a",
		Version:         "v1",
		Kind:            "Deployment",
	})
	store.Upsert(collect.Item{
		TargetNamespace: "tenant-b",
		TargetName:      "deploys",
		UID:             "b1",
		Namespace:       "tenant-b",
		Name:            "app-b",
		Version:         "v1",
		Kind:            "Deployment",
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team-inventory",
			Namespace: "tenant-a",
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).WithStatusSubresource(inv).Build()
	rec := &KollectInventoryReconciler{
		Client: cl,
		Scheme: scheme,
		Store:  store,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "tenant-a"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	key := types.NamespacedName{Name: "team-inventory", Namespace: "tenant-a"}
	if err := cl.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if got.Status.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1 (tenant-b items must not leak)", got.Status.ItemCount)
	}
}
