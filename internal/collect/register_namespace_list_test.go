// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"sync/atomic"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Reconcilers re-register their targets on every pass, and PERF-FIX-05 makes every Ready
// KollectTarget reconcile on a timer. RegisterTarget used to refresh the namespace cache
// unconditionally, which turned each of those passes into a live, cluster-wide,
// unpaginated namespace LIST per target — the cost this test pins to zero for callers
// that resolved the namespace set themselves (review finding F2).
//
// The recompute branch still has to LIST: it reads the cache it is about to match
// against, so a stale snapshot there silently drops objects in newly created namespaces.
func TestRegisterTargetNamespaceListIsOnlyPaidOnTheRecomputeBranch(t *testing.T) {
	t.Parallel()

	engine, _, profile := newScopeTransitionEngine(t)

	kube, ok := engine.kube.(*kubefake.Clientset)
	if !ok {
		t.Fatalf("engine.kube = %T, want *kubefake.Clientset", engine.kube)
	}

	var namespaceLists atomic.Int64
	kube.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		namespaceLists.Add(1)

		return false, nil, nil // fall through to the tracker
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	supplied := scopeTransitionTarget("supplied-ns", "team-a")
	if err := engine.RegisterTarget(ctx, supplied, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("register with supplied namespaces: %v", err)
	}
	if got := namespaceLists.Load(); got != 0 {
		t.Fatalf("namespace LISTs with EffectiveNamespaces supplied = %d, want 0", got)
	}

	// A re-registration is the resync shape: same state, still no LIST.
	if err := engine.RegisterTarget(ctx, supplied, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("re-register with supplied namespaces: %v", err)
	}
	if got := namespaceLists.Load(); got != 0 {
		t.Fatalf("namespace LISTs after a resync re-registration = %d, want 0", got)
	}

	recomputed := scopeTransitionTarget("recomputed-ns", "team-b")
	if err := engine.RegisterTarget(ctx, recomputed, profile, RegisterTargetOptions{}); err != nil {
		t.Fatalf("register without namespaces: %v", err)
	}
	if got := namespaceLists.Load(); got != 1 {
		t.Fatalf("namespace LISTs on the recompute branch = %d, want 1", got)
	}
}
