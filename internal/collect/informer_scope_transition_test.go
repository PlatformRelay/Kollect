// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestEngineRegisterTargetWidensRunningInformerScope(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	targetA := scopeTransitionTarget("target-a", "team-a")
	if err := engine.RegisterTarget(ctx, targetA, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("register target A: %v", err)
	}
	waitForTargetItems(t, engine, store, targetA.Namespace, targetA.Name, 1)

	targetB := scopeTransitionTarget("target-b", "team-b")
	if err := engine.RegisterTarget(ctx, targetB, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-b"},
	}); err != nil {
		t.Fatalf("register target B: %v", err)
	}
	waitForTargetItems(t, engine, store, targetB.Namespace, targetB.Name, 1)

	if got := engine.informerScope(profileGVR()); got != metav1.NamespaceAll {
		t.Fatalf("informer scope = %q, want all namespaces", got)
	}

	engine.UnregisterTarget(targetB.Namespace, targetB.Name)
	obj := scopeTransitionDeployment("team-a", "second-a", "uid-a-2")
	if _, err := engine.dynamic.Resource(profileGVR()).Namespace("team-a").Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create second team-a object: %v", err)
	}
	waitForTargetItems(t, engine, store, targetA.Namespace, targetA.Name, 2)
	if got := store.CountForTarget(targetB.Namespace, targetB.Name); got != 0 {
		t.Fatalf("removed target item count = %d, want 0", got)
	}
}

func TestEngineRegisterTargetWidensSelector(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	target := scopeTransitionTarget("target", "team-a")
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("register narrow target: %v", err)
	}
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 1)

	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("widen target: %v", err)
	}
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 2)
}

func TestEngineConcurrentTargetRegistrationConvergesOnWidenedScope(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	targetA := scopeTransitionTarget("concurrent-a", "team-a")
	targetB := scopeTransitionTarget("concurrent-b", "team-b")
	errs := make(chan error, 2)
	go func() {
		errs <- engine.RegisterTarget(ctx, targetA, profile, RegisterTargetOptions{
			EffectiveNamespaces: []string{"team-a"},
		})
	}()
	go func() {
		errs <- engine.RegisterTarget(ctx, targetB, profile, RegisterTargetOptions{
			EffectiveNamespaces: []string{"team-b"},
		})
	}()

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent RegisterTarget: %v", err)
		}
	}
	waitForTargetItems(t, engine, store, targetA.Namespace, targetA.Name, 1)
	waitForTargetItems(t, engine, store, targetB.Namespace, targetB.Name, 1)

	if got := engine.informerScope(profileGVR()); got != metav1.NamespaceAll {
		t.Fatalf("informer scope = %q, want all namespaces", got)
	}
}

func newScopeTransitionEngine(t *testing.T) (*Engine, *Store, *kollectdevv1alpha1.KollectProfile) {
	t.Helper()

	gvr := profileGVR()
	listKinds := map[schema.GroupVersionResource]string{gvr: "DeploymentList"}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
		scopeTransitionDeployment("team-a", "first-a", "uid-a-1"),
		scopeTransitionDeployment("team-b", "first-b", "uid-b-1"),
	)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}},
	)
	kube.PrependReactor("create", "selfsubjectaccessreviews", allowAllSARReactor)

	store := NewStore()
	engine, err := NewEngine(dyn, kube, store, EngineConfig{DispatchWorkers: 2, DispatchQueueSize: 16})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	profile := &kollectdevv1alpha1.KollectProfile{Spec: kollectdevv1alpha1.KollectProfileSpec{
		TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
	}}

	return engine, store, profile
}

func allowAllSARReactor(action k8stesting.Action) (bool, runtime.Object, error) {
	review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
	review.Status.Allowed = true

	return true, review, nil
}

func profileGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func scopeTransitionTarget(name, namespace string) *kollectdevv1alpha1.KollectTarget {
	return &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
	}
}

func scopeTransitionDeployment(namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"namespace": namespace,
			"name":      name,
			"uid":       uid,
		},
	}}
}

func waitForTargetItems(t *testing.T, engine *Engine, store *Store, namespace, name string, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := store.CountForTarget(namespace, name); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("target %s/%s item count = %d, want %d (forbidden=%v accessErr=%v scope=%q queue=%d)",
		namespace, name, store.CountForTarget(namespace, name), want, engine.HasForbiddenScope(namespace, name),
		engine.HasAccessCheckFailure(namespace, name), engine.informerScope(profileGVR()), len(engine.dispatchCh))
}
