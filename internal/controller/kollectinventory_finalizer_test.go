// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/metrics"
	"github.com/platformrelay/kollect/internal/sink"
)

type failingRelationalBackend struct {
	err error
}

func (f *failingRelationalBackend) Type() string { return "relational-failing" }

func (f *failingRelationalBackend) Capabilities() sink.Capabilities {
	return sink.RelationalStoreCapabilities()
}

func (f *failingRelationalBackend) Export(context.Context, []byte, string) error {
	return f.err
}

type relationalRecordingBackend struct {
	exported [][]byte
}

func (r *relationalRecordingBackend) Type() string { return "relational-recording" }

func (r *relationalRecordingBackend) Capabilities() sink.Capabilities {
	return sink.RelationalStoreCapabilities()
}

func (r *relationalRecordingBackend) Export(_ context.Context, payload []byte, _ string) error {
	r.exported = append(r.exported, append([]byte(nil), payload...))

	return nil
}

func TestKollectInventoryReconciler_addsCleanupFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	rec := &KollectInventoryReconciler{
		Client: cl,
		Scheme: scheme,
		Store:  collect.NewStore(),
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	if !containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
		t.Fatalf("finalizers = %v, want %q", got.Finalizers, inventoryCleanupFinalizer)
	}
}

func TestKollectInventoryReconciler_deleteExportsEmptyAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "web",
		UID:             "uid-1",
		Namespace:       "default",
		Name:            "demo",
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

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &relationalRecordingBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(recorder.exported) != 1 {
		t.Fatalf("cleanup export count = %d, want 1", len(recorder.exported))
	}
	if !strings.Contains(string(recorder.exported[0]), `"items":[]`) {
		t.Fatalf("cleanup payload = %s, want empty items envelope", recorder.exported[0])
	}

	var got kollectdevv1alpha1.KollectInventory
	err = cl.Get(context.Background(), types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got)
	if err == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after cleanup: %v", got.Finalizers)
		}
	}
}

// EC-P1-03: a terminal cleanup failure must not requeue (nil error, empty
// result) and must keep the finalizer in place for manual intervention.
func TestKollectInventoryReconciler_terminalCleanupDoesNotRequeue(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &failingRelationalBackend{
			err: kollecterrors.Terminal(errors.New("table schema is invalid")),
		}, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil (terminal cleanup must not requeue)", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("Reconcile result = %+v, want empty result (no requeue)", result)
	}

	var got kollectdevv1alpha1.KollectInventory
	if getErr := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); getErr != nil {
		t.Fatalf("Get inventory: %v", getErr)
	}
	if !containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
		t.Fatalf("finalizer removed despite failed cleanup: %v", got.Finalizers)
	}

	degraded := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || degraded.Reason != reasonCleanupTerminal {
		t.Fatalf("Degraded condition = %+v, want True with reason %q", degraded, reasonCleanupTerminal)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupTerminal) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupTerminal)
		}
	default:
		t.Fatalf("expected %s warning event", reasonCleanupTerminal)
	}
}

// K-29: a sink deleted before its inventory (namespace cascade does this in
// arbitrary order) must not wedge the inventory in Terminating: a missing sink
// during cleanup is "nothing to clean", cleanup completes, finalizer is removed.
func TestKollectInventoryReconciler_deleteWithMissingSinkRemovesFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("gone"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: sink.NewRegistry(),
		Recorder: recorder,
	}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil (missing sink is nothing to clean)", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("Reconcile result = %+v, want empty result (no requeue)", result)
	}

	var got kollectdevv1alpha1.KollectInventory
	if getErr := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); getErr == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after missing-sink cleanup: %v", got.Finalizers)
		}
	}

	// Loud-but-non-blocking: the sink CR is gone so its exported objects may be
	// retained in the backend; that must be announced, not swallowed.
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupSinkGone) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupSinkGone)
		}
	default:
		t.Fatalf("expected %s warning event", reasonCleanupSinkGone)
	}
}

// EC-P1-03 counterpart: transient cleanup failures keep the error-driven retry.
func TestKollectInventoryReconciler_transientCleanupStillRetries(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &failingRelationalBackend{
			err: kollecterrors.Transient(errors.New("connection refused")),
		}, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
	}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err == nil {
		t.Fatal("Reconcile err = nil, want transient cleanup error to drive retry")
	}

	var got kollectdevv1alpha1.KollectInventory
	if getErr := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); getErr != nil {
		t.Fatalf("Get inventory: %v", getErr)
	}
	if !containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
		t.Fatalf("finalizer removed despite failed cleanup: %v", got.Finalizers)
	}
}

// --- K-28 / C-2: tombstone and retention assertions -------------------------

// retentionSnapshotBackend is a snapshot sink without ExportCleaner: cleanup
// must announce retention loudly instead of the old silent success (K-28).
type retentionSnapshotBackend struct{}

func (r *retentionSnapshotBackend) Type() string { return "git" }

func (r *retentionSnapshotBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (r *retentionSnapshotBackend) Export(context.Context, []byte, string) error { return nil }

// tombstoneBackend implements sink.ExportCleaner: inventory deletion must reach
// DeleteExport with the canonical object path (C-2a tombstone/delete).
type tombstoneBackend struct {
	deleted [][]string
}

func (t *tombstoneBackend) Type() string { return "git" }

func (t *tombstoneBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (t *tombstoneBackend) Export(context.Context, []byte, string) error { return nil }

func (t *tombstoneBackend) DeleteExport(_ context.Context, paths []string) ([]string, error) {
	cp := append([]string(nil), paths...)
	t.deleted = append(t.deleted, cp)

	return cp, nil
}

func deletingInventoryWithSnapshotSink(sinkName string) (*kollectdevv1alpha1.KollectSnapshotSink, *kollectdevv1alpha1.KollectInventory) {
	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
		},
	}
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList(sinkName),
		},
	}

	return sinkObj, inv
}

func TestKollectInventoryReconciler_tombstoneCleanupDeletesExportedObjects(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj, inv := deletingInventoryWithSnapshotSink("git-demo")
	// Explicit document mode: the silent tombstone path (with layout.mode unset,
	// an auto-upgraded per-resource tree cannot be ruled out and cleanup must
	// announce retention — see implicitTreeUpgradeAnnouncesRetention).
	sinkObj.Spec.Layout = &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	tomb := &tombstoneBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGit, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return tomb, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// A retracted document-mode export must NOT announce retention.
	select {
	case ev := <-recorder.Events:
		t.Fatalf("unexpected event on a clean tombstone: %q", ev)
	default:
	}

	if len(tomb.deleted) != 1 {
		t.Fatalf("DeleteExport calls = %d, want 1: %v", len(tomb.deleted), tomb.deleted)
	}
	// Git sinks default to YAML serialization (ADR-0419): the tombstone must
	// address exactly the paths the export derived — the YAML document plus the
	// per-set manifest sidecar, not the .json request identity.
	want := []string{
		"inventory/default/team-inventory.yaml",
		"inventory/default/team-inventory.manifest.json",
	}
	if len(tomb.deleted[0]) != len(want) || tomb.deleted[0][0] != want[0] || tomb.deleted[0][1] != want[1] {
		t.Fatalf("DeleteExport paths = %v, want %v", tomb.deleted[0], want)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); err == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after tombstone cleanup: %v", got.Finalizers)
		}
	}
}

func TestKollectInventoryReconciler_retentionBackendAnnouncesAndCompletes(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj, inv := deletingInventoryWithSnapshotSink("git-demo")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGit, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &retentionSnapshotBackend{}, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); err == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after announced retention: %v", got.Finalizers)
		}
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupRetained) ||
			!strings.Contains(ev, "inventory/default/team-inventory.json") {
			t.Fatalf("event = %q, want %s naming the retained path", ev, reasonCleanupRetained)
		}
	default:
		t.Fatalf("expected %s warning event", reasonCleanupRetained)
	}
}

// K-28: with layout.mode unset, a git export may have auto-upgraded to the
// per-resource tree and cleanup cannot rule it out from the deleting side;
// retraction must be announced even when every candidate path was deleted.
func TestKollectInventoryReconciler_implicitTreeUpgradeAnnouncesRetention(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj, inv := deletingInventoryWithSnapshotSink("git-demo")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	cleaner := &tombstoneBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGit, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return cleaner, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); err == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer still present after announced retention: %v", got.Finalizers)
		}
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupRetained) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupRetained)
		}
	default:
		t.Fatalf("expected %s warning event for an auto-upgraded tree export", reasonCleanupRetained)
	}
}

// K-28: a gitlab sink in merge_request mode lands the retraction on the target
// branch only when the deletion MR is merged, so cleanup must announce
// retention even in explicit document mode.
func TestKollectInventoryReconciler_gitlabMRModeAnnouncesRetention(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "gitlab-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGitLab,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Layout: &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModeDocument},
			},
			GitLab: &kollectdevv1alpha1.GitLabSpec{
				MergeRequest: &kollectdevv1alpha1.MergeRequestSpec{Mode: "merge_request"},
			},
		},
	}
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("gitlab-demo"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	cleaner := &tombstoneBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeGitLab, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return cleaner, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(cleaner.deleted) != 1 {
		t.Fatalf("DeleteExport calls = %v, want the document retraction attempted", cleaner.deleted)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupRetained) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupRetained)
		}
	default:
		t.Fatalf("expected %s warning event for merge-request-mediated retraction", reasonCleanupRetained)
	}
}

// K-28: a {generation} path template leaves one object per past generation on
// object stores; cleanup addresses only the current one and must announce that
// a full retraction is not proven.
func TestKollectInventoryReconciler_generationTemplateAnnouncesRetention(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeS3,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				PathTemplate: "inventory/{namespace}/{name}-{generation}.json",
			},
		},
	}
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("s3-demo"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	cleaner := &tombstoneBackend{}
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeS3, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return cleaner, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(cleaner.deleted) != 1 || len(cleaner.deleted[0]) != 1 {
		t.Fatalf("DeleteExport calls = %v, want the single rendered current-generation path", cleaner.deleted)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonCleanupRetained) {
			t.Fatalf("event = %q, want reason %q", ev, reasonCleanupRetained)
		}
	default:
		t.Fatalf("expected %s warning event for stale {generation} objects", reasonCleanupRetained)
	}
}

// --- K-30: terminal cleanup observability + escape hatch --------------------

func TestKollectInventoryReconciler_terminalCleanupIncrementsMetric(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		return &failingRelationalBackend{
			err: kollecterrors.Terminal(errors.New("authentication rejected")),
		}, nil
	})

	counter := metrics.CleanupTerminalTotal.WithLabelValues("inventory")
	before := testutil.ToFloat64(counter)

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: record.NewFakeRecorder(10),
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Fatalf("kollect_cleanup_terminal_total{kind=inventory} delta = %v, want 1", got)
	}
}

// K-30 escape hatch: force-cleanup drops the finalizer without backend contact.
func TestKollectInventoryReconciler_forceCleanupAnnotationDropsFinalizer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-demo", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-inventory",
			Namespace:         "default",
			Finalizers:        []string{inventoryCleanupFinalizer},
			DeletionTimestamp: &now,
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationForceCleanup: kollectdevv1alpha1.ForceCleanupTrue,
			},
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("postgres-demo"),
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(sinkObj, inv).
		Build()

	calls := 0
	reg := sink.NewRegistry()
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext,
	) (sink.Backend, error) {
		calls++

		return &failingRelationalBackend{
			err: kollecterrors.Terminal(errors.New("revoked token")),
		}, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    collect.NewStore(),
		Registry: reg,
		Recorder: recorder,
	}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "team-inventory", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %+v, want empty", result)
	}
	if calls != 0 {
		t.Fatalf("backend constructed %d times, want 0 (force-cleanup must not contact backends)", calls)
	}

	var got kollectdevv1alpha1.KollectInventory
	if getErr := cl.Get(context.Background(),
		types.NamespacedName{Name: "team-inventory", Namespace: "default"}, &got); getErr == nil {
		if containsFinalizer(got.Finalizers, inventoryCleanupFinalizer) {
			t.Fatalf("finalizer survived force-cleanup: %v", got.Finalizers)
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
