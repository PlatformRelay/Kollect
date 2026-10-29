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

// TestProcessDispatch_reregisterDuringDispatchKeepsReplacementRow pins the race where a
// profile or target edit lands while a dispatch is between its target snapshot and
// store.Upsert. RegisterTarget replaces the registration in place and does not clear
// the store, so the backfill for the new fingerprint can write the new row first.
// The in-flight dispatch must not put the previous attributes back: nothing else
// will remove that row until the object changes again. The unregister-only case is
// TestProcessDispatch_unregisterDuringDispatchLeavesNoRow.
//
// The SubjectAccessReview reactor is the interleaving point, as in that test. It
// does not call RegisterTarget. That would enqueue the backfill on a dispatch
// worker, and the worker's own access check cannot start until this reactor
// returns, because the fake client holds its lock across the reactor. The backfill
// would then write after the stale upsert and hide the bug. The reactor applies
// the two effects that are already visible once that backfill has finished: the
// live registration's fingerprint changes, and the store holds the replacement row.
func TestProcessDispatch_reregisterDuringDispatchKeepsReplacementRow(t *testing.T) {
	gvr := profileGVR()
	listKinds := map[schema.GroupVersionResource]string{gvr: "DeploymentList"}
	object := scopeTransitionDeployment("team-b", "first-b", "uid-b-1")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, object.DeepCopy())
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}})

	target := scopeTransitionTarget("target-b", "team-b")
	target.Generation = 1
	oldProfile := deploymentProfile(1, kollectdevv1alpha1.AttributeSpec{Name: "name", Path: "{.metadata.name}"})
	newProfile := deploymentProfile(2, kollectdevv1alpha1.AttributeSpec{Name: "uid", Path: "{.metadata.uid}"})

	var engine *Engine
	var armed, replaced atomic.Bool
	var hookErr atomic.Value
	var wantUID atomic.Value
	kube.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		if !armed.CompareAndSwap(true, false) {
			return true, review, nil
		}

		key := targetKey(target.Namespace, target.Name)
		engine.mu.Lock()
		st, ok := engine.targets[key]
		if !ok {
			engine.mu.Unlock()
			hookErr.Store("target missing at re-register")

			return true, review, nil
		}
		before := st.fingerprint
		st.target.Generation = 2
		st.profile = *newProfile.DeepCopy()
		st.fingerprint = targetStateFingerprint(&st.target, &st.profile, []string{"team-b"})
		if st.fingerprint == before {
			engine.mu.Unlock()
			hookErr.Store("re-register left the fingerprint unchanged")

			return true, review, nil
		}
		engine.targets[key] = st
		engine.mu.Unlock()

		attrs, err := engine.extractor.Extract(object, newProfile.Spec.Attributes)
		if err != nil {
			hookErr.Store(err.Error())

			return true, review, nil
		}
		uid, ok := attrs["uid"]
		if !ok {
			hookErr.Store("replacement extract missed uid")

			return true, review, nil
		}
		wantUID.Store(uid)
		engine.store.Upsert(Item{
			TargetNamespace: target.Namespace,
			TargetName:      target.Name,
			Namespace:       object.GetNamespace(),
			Name:            object.GetName(),
			Group:           newProfile.Spec.TargetGVK.Group,
			Version:         newProfile.Spec.TargetGVK.Version,
			Kind:            newProfile.Spec.TargetGVK.Kind,
			UID:             string(object.GetUID()),
			Attributes:      attrs,
		})
		replaced.Store(true)

		return true, review, nil
	})

	store := NewStore()
	var err error
	engine, err = NewEngine(dyn, kube, store, EngineConfig{DispatchWorkers: 1, DispatchQueueSize: 16})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := engine.RegisterTarget(ctx, target, oldProfile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-b"},
	}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 1)
	initial := store.SnapshotTarget(target.Namespace, target.Name)
	if len(initial) != 1 || initial[0].Attributes["name"] != "first-b" {
		t.Fatalf("setup: initial attributes = %#v, want name=first-b", initial)
	}

	engine.access.Invalidate()
	armed.Store(true)
	engine.processDispatch(ctx, gvr, object, false)

	if v := hookErr.Load(); v != nil {
		t.Fatalf("re-register hook: %s", v.(string))
	}
	if !replaced.Load() {
		t.Fatal("the access check never ran, so the target was never re-registered mid-dispatch")
	}
	got := store.SnapshotTarget(target.Namespace, target.Name)
	if len(got) != 1 {
		t.Fatalf("item count = %d, want 1", len(got))
	}
	if _, ok := got[0].Attributes["name"]; ok {
		t.Fatalf("stale dispatch restored pre-change attributes %#v", got[0].Attributes)
	}
	if got[0].Attributes["uid"] != wantUID.Load() {
		t.Fatalf("attributes = %#v, want uid %#v", got[0].Attributes, wantUID.Load())
	}
}

func deploymentProfile(generation int64, attr kollectdevv1alpha1.AttributeSpec) *kollectdevv1alpha1.KollectProfile {
	return &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Generation: generation},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK:  kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{attr},
		},
	}
}
