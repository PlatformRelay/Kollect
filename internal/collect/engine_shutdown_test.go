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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestEngineDispatchWorkersStopOnContextCancel proves REL-04: when the manager
// context passed to Start is cancelled, every dispatch worker returns within a
// bounded wait, a job that was mid-flight (blocked inside the access check when
// the cancel arrives) still runs to completion, and concurrent producers never
// panic (dispatchCh is never closed).
//
// The access-check SAR reactor doubles as a mid-flight seam: it signals that a
// worker has entered processDispatch, then blocks until the test releases it.
func TestEngineDispatchWorkersStopOnContextCancel(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{}, 1)
	release := make(chan struct{})

	gvr := profileGVR()
	listKinds := map[schema.GroupVersionResource]string{gvr: "DeploymentList"}
	// No seed objects: the only matching object is the one the test creates, so
	// exactly one worker enters the gated access check and blocks mid-flight.
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
	)
	kube.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		review.Status.Allowed = true

		return true, review, nil
	})

	store := NewStore()
	engine, err := NewEngine(dyn, kube, store, EngineConfig{DispatchWorkers: 2, DispatchQueueSize: 16})
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

	target := scopeTransitionTarget("target-a", "team-a")
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}

	obj := scopeTransitionDeployment("team-a", "first-a", "uid-a-1")
	if _, err := engine.dynamic.Resource(gvr).Namespace("team-a").Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create object: %v", err)
	}

	// Wait until a worker is genuinely mid-flight (blocked inside the access check).
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("no dispatch worker entered the access check")
	}

	// Concurrent producers across the cancel boundary: objects in a namespace the
	// target does not collect are dropped before the access gate, so they neither
	// block on release nor change team-a's item count — they only exercise the
	// dispatchCh send path to prove it never panics (send-on-closed).
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		other := scopeTransitionDeployment("other", "noise", "uid-other")
		for {
			select {
			case <-ctx.Done():
				// Keep hammering a little past cancel to catch a close race, then stop.
				engine.dispatch(ctx, gvr, other, false)
				return
			default:
				engine.dispatch(ctx, gvr, other, false)
			}
		}
	}()

	// Cancel mid-flight, then release the blocked worker.
	cancel()
	close(release)

	// (a) The mid-flight job completes even though ctx was cancelled while it ran.
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 1)

	// (b) Every dispatch worker returns within a bounded wait.
	done := make(chan struct{})
	go func() {
		engine.workersWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch workers did not exit within 2s of ctx cancel")
	}

	<-producerDone
}
