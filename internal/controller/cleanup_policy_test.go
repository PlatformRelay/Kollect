// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

func cleanupPolicyScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	return scheme
}

// gitRegistryCounting registers backend for the git type and counts builds.
func gitRegistryCounting(backend sink.Backend, built *int) *sink.Registry {
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGit, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		*built++

		return backend, nil
	})

	return reg
}

func drainEvents(recorder *record.FakeRecorder) []string {
	var events []string
	for {
		select {
		case ev := <-recorder.Events:
			events = append(events, ev)
		default:
			return events
		}
	}
}

func assertFinalizerReleased(t *testing.T, cl client.Client, key types.NamespacedName, obj client.Object, finalizer string) {
	t.Helper()

	if err := cl.Get(context.Background(), key, obj); err == nil && containsFinalizer(obj.GetFinalizers(), finalizer) {
		t.Fatalf("finalizer %q still present: %v", finalizer, obj.GetFinalizers())
	}
}

// ADR-0421 default: a snapshot sink created without spec.deletionPolicy keeps
// the deleting inventory's exported objects. The backend is never built, the
// finalizer is released, and a Normal event says the exports were retained by
// policy.
func TestKollectInventoryReconciler_defaultRetainPolicyKeepsExports(t *testing.T) {
	t.Parallel()

	scheme := cleanupPolicyScheme(t)
	sinkObj, inv := deletingInventoryWithSnapshotSink("git-retain")
	sinkObj.Spec.DeletionPolicy = ""

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj, inv).WithStatusSubresource(sinkObj, inv).Build()

	tomb := &tombstoneBackend{}
	built := 0
	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client: cl, Scheme: scheme, Store: collect.NewStore(),
		Registry: gitRegistryCounting(tomb, &built), Recorder: recorder,
	}

	key := types.NamespacedName{Name: inv.Name, Namespace: inv.Namespace}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if built != 0 || len(tomb.deleted) != 0 {
		t.Fatalf("Retain must not contact the backend: built=%d DeleteExport calls=%d", built, len(tomb.deleted))
	}
	assertFinalizerReleased(t, cl, key, &kollectdevv1alpha1.KollectInventory{}, inventoryCleanupFinalizer)

	events := drainEvents(recorder)
	if len(events) != 1 || !strings.HasPrefix(events[0], corev1.EventTypeNormal) ||
		!strings.Contains(events[0], reasonCleanupRetainedByPolicy) || !strings.Contains(events[0], "git-retain") {
		t.Fatalf("events = %q, want one Normal %s naming the sink", events, reasonCleanupRetainedByPolicy)
	}
}

// MR-01: an inventory whose status shows a past export to the sink but no
// recorded paths (exported before lastExportPaths existed) cannot prove the
// retraction addressed everything: the deletion still runs, but retention is
// announced instead of a clean tombstone.
func TestKollectInventoryReconciler_unrecordedPastExportAnnouncesRetention(t *testing.T) {
	t.Parallel()

	scheme := cleanupPolicyScheme(t)
	sinkObj, inv := deletingInventoryWithSnapshotSink("git-pre-upgrade")
	sinkObj.Spec.Layout = &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument}
	exported := metav1.Now()
	inv.Status.SinkExports = []kollectdevv1alpha1.InventorySinkExportStatus{{
		Name:           kollectdevv1alpha1.SinkFamilySnapshot + "/git-pre-upgrade",
		LastExportTime: &exported,
	}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj, inv).WithStatusSubresource(sinkObj, inv).Build()

	tomb := &tombstoneBackend{}
	built := 0
	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client: cl, Scheme: scheme, Store: collect.NewStore(),
		Registry: gitRegistryCounting(tomb, &built), Recorder: recorder,
	}

	key := types.NamespacedName{Name: inv.Name, Namespace: inv.Namespace}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(tomb.deleted) != 1 {
		t.Fatalf("DeleteExport calls = %d, want 1", len(tomb.deleted))
	}
	events := drainEvents(recorder)
	if len(events) != 1 || !strings.Contains(events[0], reasonCleanupRetained) {
		t.Fatalf("events = %q, want one %s warning", events, reasonCleanupRetained)
	}
}

// MR-09: KollectInventory X in namespace "cluster" and KollectClusterInventory X
// both export as inventory/cluster/X. Deleting the namespaced one must not
// retract the cluster inventory's export.
func TestKollectInventoryReconciler_sharedClusterIdentitySkipsRetraction(t *testing.T) {
	t.Parallel()

	scheme := cleanupPolicyScheme(t)
	sinkObj, inv := deletingInventoryWithSnapshotSink("git-shared")
	sinkObj.Namespace = "cluster"
	inv.Namespace = "cluster"
	inv.Name = "platform"
	other := &kollectdevv1alpha1.KollectClusterInventory{ObjectMeta: metav1.ObjectMeta{Name: "platform"}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj, inv, other).WithStatusSubresource(sinkObj, inv).Build()

	tomb := &tombstoneBackend{}
	built := 0
	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client: cl, Scheme: scheme, Store: collect.NewStore(),
		Registry: gitRegistryCounting(tomb, &built), Recorder: recorder,
	}

	key := types.NamespacedName{Name: inv.Name, Namespace: inv.Namespace}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(tomb.deleted) != 0 {
		t.Fatalf("DeleteExport must not run for a shared identity: %v", tomb.deleted)
	}
	assertFinalizerReleased(t, cl, key, &kollectdevv1alpha1.KollectInventory{}, inventoryCleanupFinalizer)

	events := drainEvents(recorder)
	if len(events) != 1 || !strings.Contains(events[0], reasonCleanupSharedIdentity) {
		t.Fatalf("events = %q, want one %s warning", events, reasonCleanupSharedIdentity)
	}
}

// MR-09, cluster side: deleting KollectClusterInventory X must not retract the
// export of a live KollectInventory X in namespace "cluster".
func TestKollectClusterInventoryReconciler_sharedNamespacedIdentitySkipsRetraction(t *testing.T) {
	t.Parallel()

	scheme := cleanupPolicyScheme(t)
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "platform",
			Finalizers:        []string{clusterInventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace:    "kollect-system",
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-shared"),
		},
	}
	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-shared", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:           kollectdevv1alpha1.SnapshotSinkTypeGit,
			DeletionPolicy: kollectdevv1alpha1.DeletionPolicyDelete,
		},
	}
	other := &kollectdevv1alpha1.KollectInventory{ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "cluster"}}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj, inv, other).WithStatusSubresource(sinkObj, inv).Build()

	tomb := &tombstoneBackend{}
	built := 0
	recorder := record.NewFakeRecorder(10)
	rec := &KollectClusterInventoryReconciler{
		Client: cl, Scheme: scheme, Store: collect.NewStore(),
		Registry: gitRegistryCounting(tomb, &built), Recorder: recorder,
	}

	key := types.NamespacedName{Name: inv.Name}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(tomb.deleted) != 0 {
		t.Fatalf("DeleteExport must not run for a shared identity: %v", tomb.deleted)
	}
	assertFinalizerReleased(t, cl, key, &kollectdevv1alpha1.KollectClusterInventory{}, clusterInventoryCleanupFinalizer)

	events := drainEvents(recorder)
	if len(events) != 1 || !strings.Contains(events[0], reasonCleanupSharedIdentity) {
		t.Fatalf("events = %q, want one %s warning", events, reasonCleanupSharedIdentity)
	}
}
