// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
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

func TestKollectClusterInventoryReconciler_exportsRollupToSink(t *testing.T) {
	t.Parallel()

	const (
		targetName = "platform-deployments"
		workloadNS = "tenant-a"
		sinkNS     = sink.DefaultSecretNamespace
	)

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetName,
		UID:             "uid-nginx",
		Namespace:       workloadNS,
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

	tenantLabel := "kollect.dev/tenant"
	tenantVal := "team-a"

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   workloadNS,
			Labels: map[string]string{tenantLabel: tenantVal},
		},
	}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: targetName},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "unused",
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{tenantLabel: tenantVal},
			},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			Conditions: []metav1.Condition{{
				Type:   conditionReady,
				Status: metav1.ConditionTrue,
				Reason: "Collecting",
			}},
		},
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{tenantLabel: tenantVal},
			},
			TargetRefs:    []string{targetName},
			SinkRefs:      kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace: sinkNS,
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, sinkObj, inv, pgSecret).
		WithStatusSubresource(target, sinkObj, inv).
		Build()

	engine, err := collect.NewEngine(nil, nil, store)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetName, []string{workloadNS})

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Engine:   engine,
		Registry: reg,
	}

	if _, recErr := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform-rollup"},
	}); recErr != nil {
		t.Fatalf("Reconcile: %v", recErr)
	}

	if len(recorder.exported) != 1 {
		t.Fatalf("export count = %d, want 1", len(recorder.exported))
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if got.Status.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", got.Status.ItemCount)
	}

	exported := findExportSucceededCondition(got.Status.Conditions)
	if exported == nil || exported.Status != metav1.ConditionTrue {
		t.Fatalf("ExportSucceeded = %#v, want True", exported)
	}
}

func findExportSucceededCondition(conditions []metav1.Condition) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == kollectdevv1alpha1.ConditionExportSucceeded {
			return &conditions[i]
		}
	}

	return nil
}
