// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestPreviewAllSinksDebouncedThreadsReconcileContext proves REL-03: the preview
// path must thread the reconcile ctx into scopeFloor (KollectScope List) and
// loadResolvedSink (sink Get) instead of using context.Background(). An
// interceptor records the liveness of the ctx handed to every client call. With
// a pre-cancelled reconcile ctx, a correctly threaded preview issues *only*
// cancelled-ctx reads; any surviving context.Background() call surfaces as a
// live-ctx read and fails the test.
//
// This is deliberately independent of whether the fake client honours
// cancellation: the assertion is on the ctx the preview hands down, not on the
// client returning a ctx error.
func TestPreviewAllSinksDebouncedThreadsReconcileContext(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	var mu sync.Mutex
	var liveCtxCalls int
	var sawCancelledList, sawCancelledGet bool

	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	recording := interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			mu.Lock()
			if ctx.Err() != nil {
				sawCancelledList = true
			} else {
				liveCtxCalls++
			}
			mu.Unlock()

			return c.List(ctx, list, opts...)
		},
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			mu.Lock()
			if ctx.Err() != nil {
				sawCancelledGet = true
			} else {
				liveCtxCalls++
			}
			mu.Unlock()

			return c.Get(ctx, key, obj, opts...)
		},
	})

	rec := &KollectInventoryReconciler{Client: recording, Scheme: scheme}

	// A single database sink binding drives both reads: scopeFloor -> scope.Load
	// (List KollectScope) and loadResolvedSink -> ResolveSink (Get sink).
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg-a"),
		},
	}

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel() // reconcile ctx already cancelled/expired

	rec.previewAllSinksDebounced(reqCtx, inv, "default/team-inventory", "fingerprint-a")

	mu.Lock()
	defer mu.Unlock()

	if liveCtxCalls != 0 {
		t.Fatalf("preview issued %d client call(s) with a live ctx; want 0 (all reads must thread the reconcile ctx)", liveCtxCalls)
	}
	if !sawCancelledList {
		t.Fatal("scopeFloor did not issue a KollectScope List with the reconcile ctx")
	}
	if !sawCancelledGet {
		t.Fatal("loadResolvedSink did not issue a sink Get with the reconcile ctx")
	}
}
