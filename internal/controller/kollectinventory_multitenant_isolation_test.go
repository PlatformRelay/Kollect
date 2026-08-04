// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// COV-90-S13 RED: two tenants exporting concurrently — one sink unreachable must
// not block the other tenant's successful export (partial-failure isolation).
func TestKollectInventoryReconciler_concurrentTenantUnreachableIsolatesSibling(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	const (
		nsA     = "tenant-a"
		nsB     = "tenant-b"
		invName = "team-inventory"
		sinkA   = "pg-a"
		sinkB   = "pg-b"
	)

	store := collect.NewStore()
	for _, pair := range []struct{ ns, uid, name string }{
		{nsA, "uid-a", "app-a"},
		{nsB, "uid-b", "app-b"},
	} {
		store.Upsert(collect.Item{
			TargetNamespace: pair.ns,
			TargetName:      "deploys",
			UID:             pair.uid,
			Namespace:       pair.ns,
			Name:            pair.name,
			Version:         "v1",
			Kind:            "Deployment",
		})
	}

	sinkObjA := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkA, Namespace: nsA, Generation: 1},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-a-secret"},
				Table:       "items_a",
			},
		},
		Status: kollectdevv1alpha1.FamilySinkStatus{
			Conditions: []metav1.Condition{{
				Type:               kollectdevv1alpha1.ConditionConnectionVerified,
				Status:             metav1.ConditionFalse,
				Reason:             reasonSinkUnreachable,
				Message:            "dial tcp: connection refused",
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
	sinkObjB := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkB, Namespace: nsB, Generation: 1},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-b-secret"},
				Table:       "items_b",
			},
		},
	}
	secretA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-a-secret", Namespace: nsA},
		Data:       map[string][]byte{"dsn": []byte("postgres://a")},
	}
	secretB := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-b-secret", Namespace: nsB},
		Data:       map[string][]byte{"dsn": []byte("postgres://b")},
	}
	invA := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: nsA, Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkA),
		},
	}
	invB := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: nsB, Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkB),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObjA, sinkObjB, secretA, secretB, invA, invB).
		WithStatusSubresource(sinkObjA, sinkObjB, invA, invB).
		Build()

	recorder := &recordingBackend{}
	reg := newPostgresRecordingRegistry(recorder)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	t.Cleanup(func() {
		sink.EvictBackendPool(nsA, sinkA)
		sink.EvictBackendPool(nsB, sinkB)
	})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	reqs := []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: invName, Namespace: nsA}},
		{NamespacedName: types.NamespacedName{Name: invName, Namespace: nsB}},
	}
	for i := range reqs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = rec.Reconcile(context.Background(), reqs[i])
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Reconcile[%d]: %v", i, err)
		}
	}

	var gotA kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: invName, Namespace: nsA}, &gotA); err != nil {
		t.Fatalf("Get tenant-a: %v", err)
	}
	degraded := apimeta.FindStatusCondition(gotA.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonSinkUnreachable {
		t.Fatalf("tenant-a Degraded = %#v, want True/%s", degraded, reasonSinkUnreachable)
	}

	var gotB kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: invName, Namespace: nsB}, &gotB); err != nil {
		t.Fatalf("Get tenant-b: %v", err)
	}
	if gotB.Status.ItemCount != 1 {
		t.Fatalf("tenant-b ItemCount = %d, want 1", gotB.Status.ItemCount)
	}
	if len(gotB.Status.SinkExports) == 0 {
		t.Fatal("tenant-b expected SinkExports after successful export")
	}
	synced := apimeta.FindStatusCondition(gotB.Status.SinkExports[0].Conditions, conditionSinkSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("tenant-b sink Synced = %#v, want True", synced)
	}

	recorder.mu.Lock()
	exportCount := len(recorder.exported)
	payload := ""
	if exportCount > 0 {
		payload = string(recorder.exported[0])
	}
	recorder.mu.Unlock()
	if exportCount != 1 {
		t.Fatalf("export calls = %d, want 1 (only tenant-b; tenant-a sink unreachable)", exportCount)
	}
	if !strings.Contains(payload, "app-b") {
		t.Fatalf("tenant-b payload missing app-b: %s", payload)
	}
	if strings.Contains(payload, "app-a") {
		t.Fatalf("tenant-b payload leaked tenant-a item: %s", payload)
	}
}
