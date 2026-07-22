// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"runtime"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestNamespaceMatchesSelector(t *testing.T) {
	t.Parallel()

	sel := &metav1.LabelSelector{
		MatchLabels: map[string]string{"team": "platform"},
	}

	if !namespaceMatchesSelector(sel, labels.Set{"team": "platform"}) {
		t.Fatal("expected match")
	}

	if namespaceMatchesSelector(sel, labels.Set{"team": "other"}) {
		t.Fatal("expected no match")
	}

	if !namespaceMatchesSelector(nil, labels.Set{}) {
		t.Fatal("nil selector should match all")
	}
}

func TestWatchNamespaceForGVRDoesNotRetainEngineReadLockWhileResolvingNamespaces(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	engine := &Engine{
		targets: map[string]targetState{
			"ops/demo": {
				target: kollectdevv1alpha1.KollectTarget{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "demo"},
				},
				profile: kollectdevv1alpha1.KollectProfile{Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				}},
			},
		},
		nsMeta: map[string]namespaceMeta{"team-a": {}},
	}

	// Hold namespace metadata so scope resolution pauses after snapshotting target state.
	// A writer must still be able to acquire e.mu; retaining an outer read lock while
	// resolving namespaces can deadlock once that writer queues ahead of a nested RLock.
	engine.nsMu.Lock()
	done := make(chan string, 1)
	go func() { done <- engine.watchNamespaceForGVR(gvr) }()

	writerAcquired := false
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if engine.mu.TryLock() {
			writerAcquired = true
			engine.mu.Unlock()
			break
		}
		runtime.Gosched()
	}
	engine.nsMu.Unlock()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scope resolution did not complete")
	}
	if !writerAcquired {
		t.Fatal("engine writer could not acquire mu while scope resolution waited on namespace metadata")
	}
}

func TestWatchNamespaceForGVR_singleNamespace(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	engine := &Engine{
		targets: map[string]targetState{
			"ops/demo": {
				target: kollectdevv1alpha1.KollectTarget{
					Spec: kollectdevv1alpha1.KollectTargetSpec{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "prod"},
						},
					},
				},
				profile: kollectdevv1alpha1.KollectProfile{
					Spec: kollectdevv1alpha1.KollectProfileSpec{
						TargetGVK: kollectdevv1alpha1.GroupVersionKind{
							Group: "apps", Version: "v1", Kind: "Deployment",
						},
					},
				},
			},
		},
		nsMeta: map[string]namespaceMeta{
			"team-a": {Labels: labels.Set{"env": "prod"}},
			"team-b": {Labels: labels.Set{"env": "dev"}},
		},
	}

	if got := engine.watchNamespaceForGVR(gvr); got != "team-a" {
		t.Fatalf("watchNamespace = %q, want team-a", got)
	}
}

func TestWatchNamespaceForGVR_allNamespaces(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	engine := &Engine{
		targets: map[string]targetState{
			"ops/a": {
				profile: kollectdevv1alpha1.KollectProfile{
					Spec: kollectdevv1alpha1.KollectProfileSpec{
						TargetGVK: kollectdevv1alpha1.GroupVersionKind{
							Group: "apps", Version: "v1", Kind: "Deployment",
						},
					},
				},
			},
		},
		nsMeta: map[string]namespaceMeta{
			"team-a": {},
			"team-b": {},
		},
	}

	if got := engine.watchNamespaceForGVR(gvr); got != metav1.NamespaceAll {
		t.Fatalf("watchNamespace = %q, want all namespaces", got)
	}
}
