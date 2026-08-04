// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// COV-90-S13 EDGE: RBAC/SAR-denied sink for one tenant records SinkForbidden
// (operator form of skipped:forbidden) while a sibling tenant still exports.
func TestKollectInventoryReconciler_multitenantSARForbiddenDoesNotBlockSibling(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	const (
		nsForbidden = "tenant-forbidden"
		nsOK        = "tenant-ok"
		invName     = "team-inventory"
		sinkName    = "warehouse"
	)

	store := collect.NewStore()
	for _, pair := range []struct{ ns, uid, name string }{
		{nsForbidden, "uid-f", "app-forbidden"},
		{nsOK, "uid-ok", "app-ok"},
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

	sinkOK := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: nsOK},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-ok"},
				Table:       "items_ok",
			},
		},
	}
	secretOK := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-ok", Namespace: nsOK},
		Data:       map[string][]byte{"dsn": []byte("postgres://ok")},
	}
	invForbidden := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: nsForbidden, Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
		},
	}
	invOK := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: nsOK, Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
		},
	}

	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkOK, secretOK, invForbidden, invOK).
		WithStatusSubresource(sinkOK, invForbidden, invOK).
		Build()

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectdatabasesinks"}
	cl := interceptor.NewClient(base, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Namespace == nsForbidden && key.Name == sinkName {
				if _, ok := obj.(*kollectdevv1alpha1.KollectDatabaseSink); ok {
					return apierrors.NewForbidden(gr, sinkName, errors.New("SAR denied"))
				}
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	recorder := &recordingBackend{}
	reg := newPostgresRecordingRegistry(recorder)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	t.Cleanup(func() {
		sink.EvictBackendPool(nsForbidden, sinkName)
		sink.EvictBackendPool(nsOK, sinkName)
	})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	reqs := []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: invName, Namespace: nsForbidden}},
		{NamespacedName: types.NamespacedName{Name: invName, Namespace: nsOK}},
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

	var gotForbidden kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: invName, Namespace: nsForbidden}, &gotForbidden); err != nil {
		t.Fatalf("Get forbidden inventory: %v", err)
	}
	degraded := apimeta.FindStatusCondition(gotForbidden.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonSinkForbidden {
		t.Fatalf("forbidden tenant Degraded = %#v, want True/%s (skipped:forbidden)", degraded, reasonSinkForbidden)
	}

	var gotOK kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: invName, Namespace: nsOK}, &gotOK); err != nil {
		t.Fatalf("Get ok inventory: %v", err)
	}
	if gotOK.Status.ItemCount != 1 {
		t.Fatalf("ok tenant ItemCount = %d, want 1", gotOK.Status.ItemCount)
	}
	if len(gotOK.Status.SinkExports) == 0 {
		t.Fatal("ok tenant expected SinkExports after successful export")
	}
	synced := apimeta.FindStatusCondition(gotOK.Status.SinkExports[0].Conditions, conditionSinkSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("ok tenant sink Synced = %#v, want True", synced)
	}

	recorder.mu.Lock()
	exportCount := len(recorder.exported)
	payload := ""
	if exportCount > 0 {
		payload = string(recorder.exported[0])
	}
	recorder.mu.Unlock()
	if exportCount != 1 {
		t.Fatalf("export calls = %d, want 1 (only ok tenant)", exportCount)
	}
	if !strings.Contains(payload, "app-ok") {
		t.Fatalf("ok tenant payload missing app-ok: %s", payload)
	}
	if strings.Contains(payload, "app-forbidden") {
		t.Fatalf("ok tenant payload leaked forbidden-tenant item: %s", payload)
	}
}

// COV-90-S13 EDGE (target scope): SAR-denied list access for one tenant records
// ScopeForbidden — the operator status form of skipped:forbidden — while a sibling
// tenant stays Ready and Collecting.
func TestKollectTargetReconciler_multitenantForbiddenScopeIsolatesSibling(t *testing.T) {
	t.Parallel()

	forbidden := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "tenant-denied", Generation: 1},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deploys"},
	}
	ok := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "tenant-allowed", Generation: 1},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deploys"},
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(forbidden, ok).
		WithStatusSubresource(forbidden, ok).
		Build()
	eventRecorder := record.NewFakeRecorder(4)
	rec := &KollectTargetReconciler{Client: cl, Recorder: eventRecorder}

	if _, err := rec.applyTargetReadyState(context.Background(), forbidden, 0, true, false, 0, ""); err != nil {
		t.Fatalf("forbidden applyTargetReadyState: %v", err)
	}
	if _, err := rec.applyTargetReadyState(context.Background(), ok, 2, false, false, 0, ""); err != nil {
		t.Fatalf("ok applyTargetReadyState: %v", err)
	}

	syncedForbidden := apimeta.FindStatusCondition(forbidden.Status.Conditions, conditionSynced)
	if syncedForbidden == nil || syncedForbidden.Reason != reasonScopeForbidden {
		t.Fatalf("denied tenant Synced = %#v, want reason %q (skipped:forbidden)", syncedForbidden, reasonScopeForbidden)
	}
	readyForbidden := apimeta.FindStatusCondition(forbidden.Status.Conditions, conditionReady)
	if readyForbidden == nil || readyForbidden.Status != metav1.ConditionTrue {
		t.Fatalf("denied tenant Ready = %#v, want True (scope degrade, not hard-fail)", readyForbidden)
	}

	syncedOK := apimeta.FindStatusCondition(ok.Status.Conditions, conditionSynced)
	if syncedOK == nil || syncedOK.Reason != "Collecting" {
		t.Fatalf("allowed tenant Synced = %#v, want Collecting", syncedOK)
	}
	readyOK := apimeta.FindStatusCondition(ok.Status.Conditions, conditionReady)
	if readyOK == nil || readyOK.Status != metav1.ConditionTrue {
		t.Fatalf("allowed tenant Ready = %#v, want True", readyOK)
	}
}
