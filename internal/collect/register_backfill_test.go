// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"
)

// TestEngineRegisterTargetBackfillsStoreWhenInformerAlreadyRunning reproduces the
// silent-collection-loss bug: a target registered (or corrected) against an
// ALREADY-RUNNING informer never sees that informer's cached objects again.
//
// Production shape: the controller resolves effectiveNamespaces from the engine's
// namespace snapshot *before* RegisterTarget refreshes it, so a target created in a
// freshly created namespace can be registered with a stale namespace set. Every
// object in the real namespace is then silently rejected by namespaceMatches. When
// the next reconcile registers the corrected set, startInformer early-returns
// (the informer already runs at the desired scope) and nothing re-dispatches the
// informer's cache — so collection stays at 0 until an object mutates or the 12h
// resync fires.
//
// The existing coverage in informer_scope_transition_test.go only exercises the
// informer REPLACE path (which does resync), never this early-return path.
func TestEngineRegisterTargetBackfillsStoreWhenInformerAlreadyRunning(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// target-a spans two namespaces, so the informer starts cluster-wide straight
	// away and no later registration can trigger a scope replace. Waiting for both
	// of its items proves every initial Add has drained the dispatch queue, so any
	// item target-b gains later can only come from a backfill.
	targetA := scopeTransitionTarget("target-a", "team-a")
	if err := engine.RegisterTarget(ctx, targetA, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("register target A: %v", err)
	}
	waitForTargetItems(t, engine, store, targetA.Namespace, targetA.Name, 2)

	// target-b lands with a stale namespace set (the namespace it actually owns is
	// missing from the engine snapshot the controller resolved against).
	targetB := scopeTransitionTarget("target-b", "team-b")
	if err := engine.RegisterTarget(ctx, targetB, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("register target B with stale namespaces: %v", err)
	}

	// The next reconcile corrects the namespace set. No informer event is fired
	// here on purpose: the team-b Deployment is quiescent, exactly as in production.
	if err := engine.RegisterTarget(ctx, targetB, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-b"},
	}); err != nil {
		t.Fatalf("re-register target B with corrected namespaces: %v", err)
	}

	waitForTargetItems(t, engine, store, targetB.Namespace, targetB.Name, 1)
}
