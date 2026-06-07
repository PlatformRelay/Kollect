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

func TestKollectTargetReconciler_addsCleanupFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "apps",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	engine, err := collect.NewEngine(nil, nil, collect.NewStore(), collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	rec := &KollectTargetReconciler{
		Client: cl,
		Scheme: scheme,
		Engine: engine,
	}

	_, err = rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "web", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "web", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get target: %v", err)
	}

	if !containsFinalizer(got.Finalizers, targetCleanupFinalizer) {
		t.Fatalf("finalizers = %v, want %q", got.Finalizers, targetCleanupFinalizer)
	}
}

func TestKollectTargetReconciler_deleteUnregistersAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "web",
		UID:             "uid-1",
		Namespace:       "default",
		Name:            "demo",
		Version:         "v1",
		Kind:            "ConfigMap",
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	now := metav1.Now()
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web",
			Namespace:         "default",
			Finalizers:        []string{targetCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "apps"},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	engine.BindClusterTargetNamespaces("web", []string{"default"})

	rec := &KollectTargetReconciler{
		Client: cl,
		Scheme: scheme,
		Engine: engine,
	}

	_, err = rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "web", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if store.CountForTarget("default", "web") != 0 {
		t.Fatalf("store count = %d, want 0", store.CountForTarget("default", "web"))
	}

	var got kollectdevv1alpha1.KollectTarget
	err = cl.Get(context.Background(), types.NamespacedName{Name: "web", Namespace: "default"}, &got)
	if err == nil && containsFinalizer(got.Finalizers, targetCleanupFinalizer) {
		t.Fatalf("finalizer still present: %v", got.Finalizers)
	}
}
