// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"sync/atomic"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestProcessDispatch_unregisterDuringDispatchLeavesNoRow pins the race behind the historical
// FLAKE-COLLECT-01 sighting ("removed target item count = 1, want 0" in
// TestEngineRegisterTargetWidensRunningInformerScope). A dispatch snapshots the registered targets,
// then checks access and extracts before it writes the row. A target unregistered in that window
// (its KollectTarget deleted while a backfill for it is still queued) must not get its row written
// back after UnregisterTarget removed it: nothing would ever remove it again.
//
// The access check is the hook: the SubjectAccessReview reactor unregisters the target while the
// dispatch is between its snapshot and its write, which is exactly the interleaving the flake hit.
func TestProcessDispatch_unregisterDuringDispatchLeavesNoRow(t *testing.T) {
	gvr := profileGVR()
	listKinds := map[schema.GroupVersionResource]string{gvr: "DeploymentList"}
	object := scopeTransitionDeployment("team-b", "first-b", "uid-b-1")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, object.DeepCopy())
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}})

	target := scopeTransitionTarget("target-b", "team-b")
	var engine *Engine
	var armed, unregistered atomic.Bool
	kube.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		if armed.CompareAndSwap(true, false) {
			engine.UnregisterTarget(target.Namespace, target.Name)
			unregistered.Store(true)
		}

		return true, review, nil
	})

	store := NewStore()
	var err error
	engine, err = NewEngine(dyn, kube, store, EngineConfig{DispatchWorkers: 1, DispatchQueueSize: 16})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	profile := &kollectdevv1alpha1.KollectProfile{Spec: kollectdevv1alpha1.KollectProfileSpec{
		TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-b"},
	}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 1)

	// Force the next dispatch through the reactor, then replay the object the way a queued
	// backfill would.
	engine.access.Invalidate()
	armed.Store(true)
	engine.processDispatch(ctx, gvr, object, false)

	if !unregistered.Load() {
		t.Fatal("the access check never ran, so the target was never unregistered mid-dispatch -- nothing was tested")
	}
	if got := store.CountForTarget(target.Namespace, target.Name); got != 0 {
		t.Fatalf("FORBIDDEN: unregistered target has %d row(s) after a dispatch that straddled UnregisterTarget, want 0", got)
	}
}
