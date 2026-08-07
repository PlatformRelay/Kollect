// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/platformrelay/kollect/internal/metrics"
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

// TestEngineRegisterTargetSkipsBackfillWhenStateUnchanged pins the cost half of the
// fix. Reconcilers re-register targets continuously (the multitenant e2e harness does
// it every 2 seconds) and a backfill is O(objects for the GVR) x O(targets for the
// GVR) through the dispatch queue, so an unconditional backfill would trade a silent
// miss for a dispatch storm. Only a changed collection-state fingerprint may pay.
func TestEngineRegisterTargetSkipsBackfillWhenStateUnchanged(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	targetA := scopeTransitionTarget("target-a", "team-a")
	if err := engine.RegisterTarget(ctx, targetA, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("register target A: %v", err)
	}
	waitForTargetItems(t, engine, store, targetA.Namespace, targetA.Name, 2)

	if got := engine.backfillDispatches.Load(); got != 0 {
		t.Fatalf("backfills after starting the informer = %d, want 0", got)
	}

	// First registration of target-b is a state change (the target is new), so it
	// must backfill exactly once.
	targetB := scopeTransitionTarget("target-b", "team-b")
	opts := RegisterTargetOptions{EffectiveNamespaces: []string{"team-b"}}
	if err := engine.RegisterTarget(ctx, targetB, profile, opts); err != nil {
		t.Fatalf("register target B: %v", err)
	}
	waitForTargetItems(t, engine, store, targetB.Namespace, targetB.Name, 1)

	if got := engine.backfillDispatches.Load(); got != 1 {
		t.Fatalf("backfills after first registration = %d, want 1", got)
	}

	// Steady state: identical registrations must be free.
	for i := range 3 {
		if err := engine.RegisterTarget(ctx, targetB, profile, opts); err != nil {
			t.Fatalf("re-register target B (%d): %v", i, err)
		}
	}

	if got := engine.backfillDispatches.Load(); got != 1 {
		t.Fatalf("backfills after 3 unchanged re-registrations = %d, want 1", got)
	}

	// A real state change must still pay for a backfill.
	if err := engine.RegisterTarget(ctx, targetB, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("widen target B: %v", err)
	}
	waitForTargetItems(t, engine, store, targetB.Namespace, targetB.Name, 2)

	if got := engine.backfillDispatches.Load(); got != 2 {
		t.Fatalf("backfills after a changed namespace set = %d, want 2", got)
	}
}

// TestEngineDispatchCountsNamespaceMismatches covers the observability half:
// rejecting an object because its namespace is outside the target's effective set
// used to be entirely silent — no log, no metric — which is why a target stuck on
// "collecting 0" was indistinguishable from an empty cluster.
func TestEngineDispatchCountsNamespaceMismatches(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	gvr := profileGVR()
	counter, err := metrics.CollectNamespaceMismatchTotal.GetMetricWithLabelValues(
		gvr.Group, gvr.Version, gvr.Resource,
	)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues: %v", err)
	}
	before := testutil.ToFloat64(counter)

	// A wide target puts the informer cluster-wide so both seeded Deployments are
	// dispatched; the narrow target then rejects the team-b one on namespace.
	wide := scopeTransitionTarget("wide-target", "team-a")
	if err := engine.RegisterTarget(ctx, wide, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("register wide target: %v", err)
	}
	waitForTargetItems(t, engine, store, wide.Namespace, wide.Name, 2)

	narrow := scopeTransitionTarget("narrow-target", "team-a")
	if err := engine.RegisterTarget(ctx, narrow, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a"},
	}); err != nil {
		t.Fatalf("register narrow target: %v", err)
	}
	waitForTargetItems(t, engine, store, narrow.Namespace, narrow.Name, 1)

	if after := testutil.ToFloat64(counter); after <= before {
		t.Fatalf("namespace mismatch counter = %v, want > %v", after, before)
	}
}
