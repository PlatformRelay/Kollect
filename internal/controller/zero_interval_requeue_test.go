// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/validation"
)

// SCR-7: a transient total export failure of a 0s inventory is retried after the watchdog;
// other intervals keep their requeue, and a terminal failure is not requeued.

type zeroIntervalRow struct {
	name     string
	interval *metav1.Duration
	want     time.Duration
}

func zeroIntervalRows() []zeroIntervalRow {
	return []zeroIntervalRow{
		{name: "0s", interval: &metav1.Duration{}, want: validation.ZeroIntervalWatchdog},
		{name: "30s (control)", interval: &metav1.Duration{Duration: 30 * time.Second}, want: 30 * time.Second},
		{name: "unset (control, CRD default 30s)", want: 30 * time.Second},
		{name: "5m (control)", interval: &metav1.Duration{Duration: 5 * time.Minute}, want: 5 * time.Minute},
	}
}

func zeroIntervalScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	return scheme
}

func TestSetInventoryDegraded_zeroIntervalRequeues(t *testing.T) {
	t.Parallel()

	for _, row := range zeroIntervalRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			inv := &kollectdevv1alpha1.KollectInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a", Generation: 1},
				Spec:       kollectdevv1alpha1.KollectInventorySpec{ExportMinInterval: row.interval},
			}
			cl := fake.NewClientBuilder().WithScheme(zeroIntervalScheme(t)).WithObjects(inv).WithStatusSubresource(inv).Build()
			r := &KollectInventoryReconciler{Client: cl}

			result, err := r.setInventoryDegraded(context.Background(), inv, 1, reasonProgressing,
				`acquire backend for "s": secret "token" not found in namespace "team-a"`)
			if err != nil {
				t.Fatalf("setInventoryDegraded: %v", err)
			}
			if result.RequeueAfter != row.want {
				t.Fatalf("RequeueAfter = %v, want %v", result.RequeueAfter, row.want)
			}
		})
	}
}

func TestClusterSetDegraded_zeroIntervalRequeues(t *testing.T) {
	t.Parallel()

	for _, row := range zeroIntervalRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			inv := &kollectdevv1alpha1.KollectClusterInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "platform", Generation: 1},
				Spec:       kollectdevv1alpha1.KollectClusterInventorySpec{ExportMinInterval: row.interval},
			}
			cl := fake.NewClientBuilder().WithScheme(zeroIntervalScheme(t)).WithObjects(inv).WithStatusSubresource(inv).Build()
			r := &KollectClusterInventoryReconciler{Client: cl}

			result, err := r.setDegraded(context.Background(), inv, reasonProgressing,
				`acquire backend for "s": secret "token" not found in namespace "kollect-system"`)
			if err != nil {
				t.Fatalf("setDegraded: %v", err)
			}
			if result.RequeueAfter != row.want {
				t.Fatalf("RequeueAfter = %v, want %v", result.RequeueAfter, row.want)
			}
		})
	}
}

func TestApplyInventoryExportOutcome_zeroIntervalTerminalNoRequeue(t *testing.T) {
	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a", TargetName: "web", UID: "uid-terminal",
		Namespace: "team-a", Name: "demo", Version: "v1", Kind: "Deployment",
	})

	snapshotSink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "zero-terminal", Namespace: "team-a", UID: "uid-zero-terminal"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: "s3"},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "zero-terminal", Namespace: "team-a", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &metav1.Duration{},
			SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("zero-terminal"),
		},
	}
	cl := fake.NewClientBuilder().WithScheme(zeroIntervalScheme(t)).
		WithObjects(snapshotSink, inv).WithStatusSubresource(inv).Build()

	reg := sink.NewRegistry()
	reg.Register("s3", func(kollectdevv1alpha1.KollectSinkSpec, sink.BuildContext) (sink.Backend, error) {
		return nil, kollecterrors.Terminal(errors.New("bad sink configuration"))
	})
	r := &KollectInventoryReconciler{Client: cl, Scheme: cl.Scheme(), Store: store, Registry: reg}
	t.Cleanup(func() { sink.EvictBackendPoolByUID("uid-zero-terminal") })

	result, err := r.applyInventoryExportOutcome(context.Background(), noopLogger{}, inv, 1, "team-a/zero-terminal",
		store.SnapshotNamespace("team-a"), "checksum")
	if err != nil {
		t.Fatalf("applyInventoryExportOutcome: %v", err)
	}
	cond := apimeta.FindStatusCondition(inv.Status.Conditions, conditionDegraded)
	if cond == nil || cond.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("degraded condition = %+v, want reason %s (the terminal total-failure branch)", cond,
			kollectdevv1alpha1.ReasonExportTerminal)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("RequeueAfter = %v, want 0: a terminal failure must not be requeued", result.RequeueAfter)
	}
}

// SCR-7: the watchdog keeps reconciling an unchanged, successfully exported 0s binding; its
// coalescing state must not age out while those reconciles keep skipping it, or a later
// reconcile after another binding's record() pruned it would export the unchanged payload again.
func TestPerSinkCoalesceTracker_zeroIntervalSkipsKeepStateAlive(t *testing.T) {
	t.Parallel()

	var tracker perSinkCoalesceTracker
	base := time.Now()
	invKey, sinkName := "team-a/zero", "git"

	tracker.record(invKey, sinkName, 1, "hash", "", base)
	for at := base; !at.After(base.Add(2 * coalesceStateTTL)); at = at.Add(coalesceStateTTL / 8) {
		if !tracker.shouldSkip(invKey, sinkName, 1, "hash", "", 0, at) {
			t.Fatalf("unchanged 0s binding not skipped at +%v", at.Sub(base))
		}
		// Another inventory exports meanwhile; its record() sweeps the tracker.
		tracker.record("team-a/other", sinkName, 1, "other", "", at)
	}
}
