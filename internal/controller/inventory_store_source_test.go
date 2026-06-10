// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// PERF-01 / EC-P1-04: store watch enqueues inventories in the changed target namespace.
func TestInventoriesInNamespace_enqueuesAllInNamespace(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	invA := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: "team-a"},
	}
	invB := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: "team-a"},
	}
	otherNS := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-b"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(invA, invB, otherNS).Build()

	reqs := inventoriesInNamespace(context.Background(), cl, "team-a")
	if len(reqs) != 2 {
		t.Fatalf("len(reqs) = %d, want 2", len(reqs))
	}

	seen := map[string]struct{}{}
	for _, req := range reqs {
		if req.Namespace != "team-a" {
			t.Fatalf("request namespace = %q", req.Namespace)
		}
		seen[req.Name] = struct{}{}
	}
	if _, ok := seen["inv-a"]; !ok {
		t.Fatalf("missing inv-a in %#v", reqs)
	}
	if _, ok := seen["inv-b"]; !ok {
		t.Fatalf("missing inv-b in %#v", reqs)
	}
}

func TestInventoriesInNamespace_emptyNamespace(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	reqs := inventoriesInNamespace(context.Background(), cl, "empty")
	if len(reqs) != 0 {
		t.Fatalf("len(reqs) = %d, want 0", len(reqs))
	}
}

func TestNewInventoryStoreSource_nonNil(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	src := newInventoryStoreSource(collect.NewStore(), cl)
	if src == nil {
		t.Fatal("expected non-nil source")
	}
}

func TestInventoryStoreSourceStart_nilDependenciesWaitsForCancel(t *testing.T) {
	t.Parallel()

	src := &inventoryNamespaceSource{}
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- src.Start(ctx, queue)
	}()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}
}

func TestInventoryStoreSourceStart_enqueuesInventoryRequests(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: "team-a"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()
	store := collect.NewStore()
	src := &inventoryNamespaceSource{store: store, reader: cl}

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- src.Start(ctx, queue)
	}()
	time.Sleep(25 * time.Millisecond)

	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "target-a",
		UID:             "uid-1",
	})

	deadline := time.Now().Add(3 * time.Second)
	for queue.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if queue.Len() == 0 {
		cancel()
		<-done
		t.Fatal("queue did not receive inventory reconcile request")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}
