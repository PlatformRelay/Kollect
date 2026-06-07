// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestKollectClusterTargetReconciler_suspend(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	store := collect.NewStore()
	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatal(err)
	}
	engine.BindClusterTargetNamespaces("cluster-deploys", []string{"team-a"})
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "cluster-deploys",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
	})

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-deploys", Generation: 1},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "demo",
			Suspend:    true,
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(ct).
		WithObjects(ct).
		Build()

	r := &KollectClusterTargetReconciler{Client: c, Scheme: scheme, Engine: engine}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: ct.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if engine.ItemCount("team-a", ct.Name) != 0 {
		t.Fatal("expected engine unregister on suspend")
	}

	var got kollectdevv1alpha1.KollectClusterTarget
	if err := c.Get(context.Background(), types.NamespacedName{Name: ct.Name}, &got); err != nil {
		t.Fatal(err)
	}

	cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Reason != "Suspended" {
		t.Fatalf("Degraded = %+v", cond)
	}
}
