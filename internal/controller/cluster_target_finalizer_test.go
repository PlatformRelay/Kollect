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

func TestKollectClusterTargetReconciler_deleteUnregistersAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "platform",
		UID:             "uid-1",
		Namespace:       "team-a",
		Name:            "demo",
		Version:         "v1",
		Kind:            "ConfigMap",
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	now := metav1.Now()
	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform",
			Finalizers:        []string{clusterTargetCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{ProfileRef: "apps"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ct).
		WithStatusSubresource(ct).
		Build()

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	engine.BindClusterTargetNamespaces("platform", []string{"team-a"})

	rec := &KollectClusterTargetReconciler{
		Client: cl,
		Scheme: scheme,
		Engine: engine,
	}

	_, err = rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if store.CountForTarget("team-a", "platform") != 0 {
		t.Fatalf("store count = %d, want 0", store.CountForTarget("team-a", "platform"))
	}

	var got kollectdevv1alpha1.KollectClusterTarget
	err = cl.Get(context.Background(), types.NamespacedName{Name: "platform"}, &got)
	if err == nil && containsFinalizer(got.Finalizers, clusterTargetCleanupFinalizer) {
		t.Fatalf("finalizer still present: %v", got.Finalizers)
	}
}
