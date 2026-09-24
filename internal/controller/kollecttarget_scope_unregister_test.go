// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// TestScopeDenyUnregistersTarget is the B3 batch test lock
// "register -> deny -> no items" for K-05: a namespaced target that was Ready
// and collecting must be dropped from the engine (store emptied, no further
// items) the moment a KollectScope denies it — mirroring what the cluster
// enforcement path already did via unregisterAll before degrading.
func TestScopeDenyUnregistersTarget(t *testing.T) {
	t.Parallel()

	const testNS = "tenant-denied"

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "configs", Namespace: testNS},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "configs-profile",
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "configs-profile", Namespace: testNS},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target, profile).
		WithStatusSubresource(&kollectdevv1alpha1.KollectTarget{}).
		Build()

	store := collect.NewStore()
	engine, _ := newScopeDenyEngine(t, testNS, store)

	r := &KollectTargetReconciler{
		Client:   cl,
		Scheme:   scheme,
		Engine:   engine,
		Recorder: record.NewFakeRecorder(10),
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "configs", Namespace: testNS}}
	rctx := context.Background()

	// Phase 1: no scope — register and collect.
	if _, err := r.Reconcile(rctx, req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if !waitForCount(store, testNS, "configs", 1, 5*time.Second) {
		t.Fatalf("phase 1: collected %d items, want 1 (harness must really deliver before the deny is meaningful)",
			store.CountForTarget(testNS, "configs"))
	}

	// Phase 2: tighten the tenancy ceiling after the target went Ready.
	denyScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "tighten", Namespace: testNS},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Version: "v1", Kind: "ConfigMap"},
				},
				AllowedNamespaces: []string{"somewhere-else"},
			},
		},
	}
	if err := cl.Create(rctx, denyScope); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(rctx, req); err != nil {
		t.Fatalf("deny reconcile: %v", err)
	}

	var updated kollectdevv1alpha1.KollectTarget
	if err := cl.Get(rctx, req.NamespacedName, &updated); err != nil {
		t.Fatal(err)
	}
	deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
	if deg == nil || deg.Reason != scopeReasonNSDenied {
		t.Fatalf("expected Degraded/%s, got %+v", scopeReasonNSDenied, deg)
	}

	if got := engine.ItemCount(testNS, "configs"); got != 0 {
		t.Fatalf("engine ItemCount after scope deny = %d, want 0 (K-05: deny must unregister)", got)
	}
	if got := store.CountForTarget(testNS, "configs"); got != 0 {
		t.Fatalf("store items after scope deny = %d, want 0 (no items may keep reaching the sink path)", got)
	}
}

// TestScopeDenyStopsLaterItems pins the "no items" half of the lock over the
// observation window: the deny must survive dispatch of objects that were
// already cached when the target was registered — a still-registered frozen
// effective set would keep accepting them.
func TestScopeDenyStopsLaterItems(t *testing.T) {
	t.Parallel()

	const testNS = "tenant-tightened"

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "configs", Namespace: testNS},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "p"},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: testNS},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target, profile).
		WithStatusSubresource(&kollectdevv1alpha1.KollectTarget{}).
		Build()

	store := collect.NewStore()
	engine, dyn := newScopeDenyEngine(t, testNS, store)

	r := &KollectTargetReconciler{
		Client:   cl,
		Scheme:   scheme,
		Engine:   engine,
		Recorder: record.NewFakeRecorder(10),
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "configs", Namespace: testNS}}
	rctx := context.Background()

	if _, err := r.Reconcile(rctx, req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if !waitForCount(store, testNS, "configs", 1, 5*time.Second) {
		t.Fatalf("phase 1: want 1 collected item, got %d", store.CountForTarget(testNS, "configs"))
	}

	denyScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: testNS},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Version: "v1", Kind: "ConfigMap"},
				},
				AllowedNamespaces: []string{"somewhere-else"},
			},
		},
	}
	if err := cl.Create(rctx, denyScope); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(rctx, req); err != nil {
		t.Fatalf("deny reconcile: %v", err)
	}

	// A late object delivered by the running informer after the deny must not
	// be stored: the target must no longer be registered at all.
	late := scopeDenyConfigMap(testNS, "late-cm", "uid-late")
	if _, err := dyn.Resource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}).
		Namespace(testNS).Create(rctx, late, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed late object: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := store.CountForTarget(testNS, "configs"); got != 0 {
		t.Fatalf("store items after deny + late object = %d, want 0", got)
	}
}

func newScopeDenyEngine(t *testing.T, testNS string, store *collect.Store) (*collect.Engine, *dynfake.FakeDynamicClient) {
	t.Helper()

	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
		scopeDenyConfigMap(testNS, "cm-1", "uid-1"),
	)
	kube := kubefake.NewSimpleClientset( //nolint:staticcheck // SimpleClientset is sufficient here
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}},
	)
	kube.PrependReactor("create", "selfsubjectaccessreviews", allowAllAccessReview)

	engine, err := collect.NewEngine(dyn, kube, store, collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engineCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(engineCtx); err != nil {
		t.Fatalf("engine Start: %v", err)
	}

	return engine, dyn
}

func allowAllAccessReview(action k8stesting.Action) (bool, runtime.Object, error) {
	review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
	review.Status.Allowed = true

	return true, review, nil
}

func scopeDenyConfigMap(namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"namespace": namespace,
			"name":      name,
			"uid":       uid,
		},
	}}
}

func waitForCount(store *collect.Store, namespace, name string, want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if store.CountForTarget(namespace, name) == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}

	return false
}
