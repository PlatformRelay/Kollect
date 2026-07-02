// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestGuardReconcile_recoversPanicAndRequeues(t *testing.T) {
	t.Parallel()

	recorder := record.NewFakeRecorder(1)
	obj := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "panic-test", Namespace: "default"},
	}

	result, err := guardReconcile(context.Background(), recorder, obj, func() (ctrl.Result, error) {
		panic("injected reconcile panic")
	})
	if err != nil {
		t.Fatalf("guardReconcile err = %v", err)
	}
	if result.RequeueAfter == 0 && !result.Requeue { //nolint:staticcheck // SA1019: guard uses Requeue for immediate requeue
		t.Fatal("expected requeue after panic")
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, "ReconcilePanic") {
			t.Fatalf("event = %q", ev)
		}
	default:
		t.Fatal("expected ReconcilePanic event")
	}
}

// EC-P2-02: the family-sink reconciler entrypoint must be panic-guarded — a
// panic inside its reconcile path is recovered, requeued, and not propagated.
// Verified for all three family-sink kinds (Snapshot/Database/Event) by the
// generic table-driven tests in family_sink_generic_test.go (AR-08).

func TestGuardReconcile_passesThroughSuccess(t *testing.T) {
	t.Parallel()

	result, err := guardReconcile(context.Background(), nil, nil, func() (ctrl.Result, error) {
		return ctrl.Result{RequeueAfter: 30}, nil
	})
	if err != nil {
		t.Fatalf("guardReconcile err = %v", err)
	}
	if result.RequeueAfter != 30 {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}
