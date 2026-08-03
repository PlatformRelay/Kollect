// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
)

// terminalFailBackend fails every Export with a TERMINAL (invalid-config) error so the
// reconciler exercises the hard-degrade, no-requeue path.
type terminalFailBackend struct{}

func (terminalFailBackend) Type() string { return "terminal" }

func (terminalFailBackend) Capabilities() sink.Capabilities { return sink.SnapshotStoreCapabilities() }

func (terminalFailBackend) Export(_ context.Context, _ []byte, _ string) error {
	return kollecterrors.Terminal(errors.New("sink misconfigured: unknown table"))
}

// TestClusterInventory_TerminalExportFailure_DegradesNoRequeue is the COV-90-S01 ErrTerminal
// EDGE lock via reconcileRollupExport (kollectclusterinventory_controller.go:205-222): a
// terminal export failure must NOT requeue (RequeueAfter==0), must set Degraded=True with
// reason ExportTerminal, and must emit a Warning Event. A retryable classification here would
// spin the reconciler forever on a permanent config fault.
func TestClusterInventory_TerminalExportFailure_DegradesNoRequeue(t *testing.T) {
	t.Parallel()

	const (
		targetName = "platform-deployments"
		workloadNS = "tenant-a"
		sinkNS     = sink.DefaultSecretNamespace
	)

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS,
		TargetName:      targetName,
		UID:             "uid-nginx",
		Namespace:       workloadNS,
		Name:            "nginx",
		Version:         "v1",
		Kind:            "Deployment",
		Attributes:      map[string]any{"image": "nginx:1.27"},
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	tenantLabel, tenantVal := "kollect.dev/tenant", "team-a"
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workloadNS, Labels: map[string]string{tenantLabel: tenantVal}}}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: targetName},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef:        kollectdevv1alpha1.NamespacedObjectReference{Name: "unused", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			Conditions: []metav1.Condition{{Type: conditionReady, Status: metav1.ConditionTrue, Reason: "Collecting"}},
		},
	}

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type:     kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"}, Table: "inventory_items"},
		},
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
			TargetRefs:        []string{targetName},
			DatabaseSinkRefs:  kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace:     sinkNS,
		},
	}

	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, sinkObj, inv, pgSecret).
		WithStatusSubresource(target, sinkObj, inv).
		Build()

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetName, []string{workloadNS})

	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return terminalFailBackend{}, nil
	})

	recorder := record.NewFakeRecorder(10)
	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Engine:   engine,
		Registry: reg,
		Recorder: recorder,
	}

	res, recErr := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "platform-rollup"}})
	if recErr != nil {
		t.Fatalf("Reconcile: %v", recErr)
	}
	if res.RequeueAfter != 0 {
		t.Fatalf("RequeueAfter = %v on a TERMINAL export failure; want 0 (no requeue for a permanent config fault)", res.RequeueAfter)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "platform-rollup"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	deg := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if deg == nil || deg.Status != metav1.ConditionTrue || deg.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("Degraded = %+v, want True/ExportTerminal", deg)
	}

	if !drainHasWarning(recorder) {
		t.Fatal("expected a Warning Event on terminal export failure; none emitted")
	}
}

// TestClusterInventory_MultipartExport_SharedPrunePlan covers the len(parts)>1 branch in
// exportClusterToSinks (kollectclusterinventory_controller.go:298-300) added by #183: when a
// per-ref byte ceiling forces a rollup into multiple parts, the reconciler builds ONE shared
// PrunePlan and drives every part through it (part-suffixed object paths).
func TestClusterInventory_MultipartExport_SharedPrunePlan(t *testing.T) {
	t.Parallel()

	const (
		targetName = "platform-deployments"
		workloadNS = "tenant-a"
		sinkNS     = sink.DefaultSecretNamespace
	)

	store := collect.NewStore()
	for i := range 3 {
		store.Upsert(collect.Item{
			TargetNamespace: workloadNS,
			TargetName:      targetName,
			UID:             fmt.Sprintf("uid-%d", i),
			Namespace:       workloadNS,
			Name:            fmt.Sprintf("app-%d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"payload": strings.Repeat("x", 220)},
		})
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	tenantLabel, tenantVal := "kollect.dev/tenant", "team-a"
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workloadNS, Labels: map[string]string{tenantLabel: tenantVal}}}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: targetName},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef:        kollectdevv1alpha1.NamespacedObjectReference{Name: "unused", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			Conditions: []metav1.Condition{{Type: conditionReady, Status: metav1.ConditionTrue, Reason: "Collecting"}},
		},
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"},
		},
	}

	limit := int64(900)
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
			TargetRefs:        []string{targetName},
			// Per-ref byte ceiling forces the rollup to partition into >1 part.
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "git-platform", MaxExportBytes: &limit}},
			SinkNamespace:    sinkNS,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, sinkObj, inv).
		WithStatusSubresource(target, sinkObj, inv).
		Build()

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetName, []string{workloadNS})

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Engine:   engine,
		Registry: reg,
	}

	if _, recErr := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "platform-rollup"}}); recErr != nil {
		t.Fatalf("Reconcile: %v", recErr)
	}

	if len(recorder.exported) < 2 {
		t.Fatalf("export count = %d, want >= 2 parts (multipart branch not exercised)", len(recorder.exported))
	}
	for i := range recorder.exported {
		if int64(len(recorder.exported[i])) > limit {
			t.Fatalf("part %d size = %d, want <= %d", i+1, len(recorder.exported[i]), limit)
		}
	}
	for _, p := range recorder.paths {
		if p == "inventory/cluster/platform-rollup.json" {
			t.Fatalf("multipart export must use part-suffixed object paths; got unsuffixed %q", p)
		}
	}
}

// clusterRollupScheme builds a scheme with the kollect + core types registered.
func clusterRollupScheme(t *testing.T) *runtime.Scheme {
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

// readyClusterFixture wires a store/engine/target/namespace with one Ready target holding a
// single snapshot item, for driving reconcileRollupExport branches that do not need a sink.
func readyClusterFixture(t *testing.T) (*collect.Store, *collect.Engine, *corev1.Namespace, *kollectdevv1alpha1.KollectClusterTarget) {
	t.Helper()
	const (
		targetName = "platform-deployments"
		workloadNS = "tenant-a"
	)
	tenantLabel, tenantVal := "kollect.dev/tenant", "team-a"

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: workloadNS, TargetName: targetName, UID: "uid-a",
		Namespace: workloadNS, Name: "app", Version: "v1", Kind: "Deployment",
		Attributes: map[string]any{"image": "nginx:1.27"},
	})

	engine, err := collect.NewEngine(nil, nil, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.BindClusterTargetNamespaces(targetName, []string{workloadNS})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workloadNS, Labels: map[string]string{tenantLabel: tenantVal}}}
	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: targetName},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef:        kollectdevv1alpha1.NamespacedObjectReference{Name: "unused", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			Conditions: []metav1.Condition{{Type: conditionReady, Status: metav1.ConditionTrue, Reason: "Collecting"}},
		},
	}

	return store, engine, ns, target
}

// TestClusterInventory_NoSinkRefs_NoExport covers the no-bindings branch of
// reconcileRollupExport (kollectclusterinventory_controller.go:194-197): an inventory with no
// sink refs records the rollup status with Synced=NoExport rather than attempting an export.
func TestClusterInventory_NoSinkRefs_NoExport(t *testing.T) {
	t.Parallel()

	scheme := clusterRollupScheme(t)
	store, engine, ns, target := readyClusterFixture(t)

	tenantLabel, tenantVal := "kollect.dev/tenant", "team-a"
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "no-sinks"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
			TargetRefs:        []string{target.Name},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, inv).
		WithStatusSubresource(target, inv).
		Build()

	rec := &KollectClusterInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Engine: engine}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "no-sinks"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "no-sinks"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	// The no-bindings branch records the rollup and never degrades on a missing sink.
	if deg := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded); deg != nil && deg.Status == metav1.ConditionTrue {
		t.Fatalf("unexpected Degraded with no sink refs: %+v", deg)
	}
	if got.Status.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1 (rollup recorded without export)", got.Status.ItemCount)
	}
	synced := apimeta.FindStatusCondition(got.Status.Conditions, conditionSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("Synced = %+v, want True (rollup recorded)", synced)
	}
}

// TestClusterInventory_NilRegistry_DegradesExportUnavailable covers the registry-nil branch
// (kollectclusterinventory_controller.go:199-201): with sinks configured but no export
// registry wired, the inventory degrades with ExportUnavailable instead of panicking.
func TestClusterInventory_NilRegistry_DegradesExportUnavailable(t *testing.T) {
	t.Parallel()

	scheme := clusterRollupScheme(t)
	store, engine, ns, target := readyClusterFixture(t)

	sinkNS := sink.DefaultSecretNamespace
	tenantLabel, tenantVal := "kollect.dev/tenant", "team-a"
	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type:     kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"}, Table: "inventory_items"},
		},
	}
	pgSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS}, Data: map[string][]byte{"dsn": []byte("postgres://example")}}
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "no-registry"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: tenantVal}},
			TargetRefs:        []string{target.Name},
			DatabaseSinkRefs:  kollectdevv1alpha1.NewSinkRefList("postgres-platform"),
			SinkNamespace:     sinkNS,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, sinkObj, pgSecret, inv).
		WithStatusSubresource(target, sinkObj, inv).
		Build()

	// Registry deliberately nil.
	rec := &KollectClusterInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Engine: engine}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "no-registry"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "no-registry"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	deg := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if deg == nil || deg.Status != metav1.ConditionTrue || deg.Reason != "ExportUnavailable" {
		t.Fatalf("Degraded = %+v, want True/ExportUnavailable", deg)
	}
}

// drainHasWarning reports whether the fake recorder emitted at least one Warning event.
func drainHasWarning(r *record.FakeRecorder) bool {
	for {
		select {
		case ev := <-r.Events:
			if strings.HasPrefix(ev, "Warning") {
				return true
			}
		default:
			return false
		}
	}
}
