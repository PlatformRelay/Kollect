// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// A non-NotFound sink-resolution failure is collected (and surfaced) instead of
// being mistaken for a vanished sink.
func TestCleanupSinkExports_ResolutionErrorIsCollected(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	report, err := cleanupSinkExports(
		context.Background(),
		cl,
		sink.NewRegistry(),
		"default",
		[]kollectdevv1alpha1.InventorySinkBinding{{Family: "bogus", Name: "x"}},
		false,
		"inventory/team-a/inv.json",
		1,
	)
	if err == nil {
		t.Fatal("unknown sink family must surface as an error")
	}
	if !report.empty() {
		t.Fatalf("report = %+v, want empty (no vanished or retained sinks)", report)
	}
}

// K-30 escape hatch: the force-cleanup annotation drops the finalizer without
// backend retraction and announces the retention loudly.
func TestKollectClusterInventoryReconciler_forceCleanupSkipsBackends(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform-rollup",
			Finalizers:        []string{clusterInventoryCleanupFinalizer},
			DeletionTimestamp: &now,
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationForceCleanup: kollectdevv1alpha1.ForceCleanupTrue,
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	recorder := record.NewFakeRecorder(10)
	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: sink.NewRegistry(),
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "platform-rollup"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if getErr := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); getErr == nil {
		if containsFinalizer(got.Finalizers, clusterInventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after forced cleanup: %v", got.Finalizers)
		}
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupForced) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupForced)
		}
	default:
		t.Fatalf("expected %s warning event", reasonCleanupForced)
	}
}
