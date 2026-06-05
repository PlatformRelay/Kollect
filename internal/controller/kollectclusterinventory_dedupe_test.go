// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

func TestKollectClusterInventoryReconciler_dedupesCrossTargetRows(t *testing.T) {
	t.Parallel()

	const (
		targetA    = "platform-deployments"
		targetB    = "platform-deployments-alt"
		workloadNS = "tenant-a"
		sinkNS     = sink.DefaultSecretNamespace
		sharedUID  = "uid-nginx"
	)

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetA,
		UID:             sharedUID,
		Namespace:       workloadNS,
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
		Attributes:      map[string]any{"replicas": 1},
	})
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetB,
		UID:             sharedUID,
		Namespace:       workloadNS,
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
		Attributes:      map[string]any{"replicas": 3},
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

	ready := []metav1.Condition{{
		Type:   conditionReady,
		Status: metav1.ConditionTrue,
		Reason: "Collecting",
	}}

	targetObjs := []*kollectdevv1alpha1.KollectClusterTarget{
		{
			ObjectMeta: metav1.ObjectMeta{Name: targetA},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantVal},
				},
			},
			Status: kollectdevv1alpha1.KollectClusterTargetStatus{Conditions: ready},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: targetB},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantVal},
				},
			},
			Status: kollectdevv1alpha1.KollectClusterTargetStatus{Conditions: ready},
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
			TargetRefs:    []string{targetA, targetB},
			SinkRefs:      kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace: sinkNS,
			Dedupe:        kollectdevv1alpha1.ClusterInventoryDedupeByResourceUID,
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	objs := make([]client.Object, 0, 4+len(targetObjs))
	objs = append(objs, ns, sinkObj, inv, pgSecret)
	for _, ct := range targetObjs {
		objs = append(objs, ct)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(targetObjs[0], targetObjs[1], sinkObj, inv).
		Build()

	engine, err := collect.NewEngine(nil, nil, store)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetA, []string{workloadNS})
	engine.BindClusterTargetNamespaces(targetB, []string{workloadNS})

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

	exported, err := collect.ItemsFromExportPayload(recorder.exported[0])
	if err != nil {
		t.Fatalf("decode export: %v", err)
	}

	if len(exported) != 1 {
		t.Fatalf("exported items = %d, want 1 (deduped by resource UID)", len(exported))
	}

	if got, ok := exported[0].Attributes["replicas"].(float64); !ok || got != 3 {
		t.Fatalf("last row wins: replicas = %v, want 3", exported[0].Attributes["replicas"])
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if got.Status.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", got.Status.ItemCount)
	}
}

func TestKollectClusterInventoryReconciler_shouldDebounce(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	invKey := "default/platform"
	sinkName := "postgres-platform"
	interval := time.Minute
	now := time.Now()
	gen := int64(1)
	hashA := "hash-a"
	hashB := "hash-b"

	if tracker.shouldSkip(invKey, sinkName, gen, hashA, interval, now) {
		t.Fatal("first export must not debounce")
	}

	tracker.record(invKey, sinkName, gen, hashA, now)
	if !tracker.shouldSkip(invKey, sinkName, gen, hashA, interval, now) {
		t.Fatal("identical payload within interval should debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, gen, hashB, interval, now) {
		t.Fatal("payload change must not debounce")
	}

	if tracker.shouldSkip(invKey, sinkName, gen+1, hashA, interval, now) {
		t.Fatal("generation bump must not debounce")
	}
}

func TestKollectClusterInventoryReconciler_keepAllPreservesCrossTargetRows(t *testing.T) {
	t.Parallel()

	const (
		targetA    = "platform-deployments"
		targetB    = "platform-deployments-alt"
		workloadNS = "tenant-a"
		sinkNS     = sink.DefaultSecretNamespace
		sharedUID  = "uid-nginx"
	)

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetA,
		UID:             sharedUID,
		Namespace:       workloadNS,
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
	})
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetB,
		UID:             sharedUID,
		Namespace:       workloadNS,
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

	tenantLabel := "kollect.dev/tenant"
	tenantVal := "team-a"

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   workloadNS,
			Labels: map[string]string{tenantLabel: tenantVal},
		},
	}

	ready := []metav1.Condition{{
		Type:   conditionReady,
		Status: metav1.ConditionTrue,
		Reason: "Collecting",
	}}

	targetObjs := []*kollectdevv1alpha1.KollectClusterTarget{
		{
			ObjectMeta: metav1.ObjectMeta{Name: targetA},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantVal},
				},
			},
			Status: kollectdevv1alpha1.KollectClusterTargetStatus{Conditions: ready},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: targetB},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantVal},
				},
			},
			Status: kollectdevv1alpha1.KollectClusterTargetStatus{Conditions: ready},
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
			TargetRefs:    []string{targetA, targetB},
			SinkRefs:      kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace: sinkNS,
			Dedupe:        kollectdevv1alpha1.ClusterInventoryDedupeKeepAll,
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	objs := make([]client.Object, 0, 4+len(targetObjs))
	objs = append(objs, ns, sinkObj, inv, pgSecret)
	for _, ct := range targetObjs {
		objs = append(objs, ct)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(targetObjs[0], targetObjs[1], sinkObj, inv).
		Build()

	engine, err := collect.NewEngine(nil, nil, store)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetA, []string{workloadNS})
	engine.BindClusterTargetNamespaces(targetB, []string{workloadNS})

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

	exported, err := collect.ItemsFromExportPayload(recorder.exported[0])
	if err != nil {
		t.Fatalf("decode export: %v", err)
	}

	if len(exported) != 2 {
		t.Fatalf("exported items = %d, want 2 (keepAll default)", len(exported))
	}
}
