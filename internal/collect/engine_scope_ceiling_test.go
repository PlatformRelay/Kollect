// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestEngineCeilingEmptyEffectiveFailsClosed is the K-06 batch test lock
// ("empty-effective fail closed") at engine level: a target that collects while
// unrestricted starts storing, then a re-registration whose ceiling leaves the
// effective set empty must stop the flow — the empty set may not fall back to
// unrestricted selector matching, and the stale items must be evicted.
func TestEngineCeilingEmptyEffectiveFailsClosed(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	target := scopeTransitionTarget("ceiling-target", "team-a")
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		EffectiveNamespaces: []string{"team-a", "team-b"},
	}); err != nil {
		t.Fatalf("register unrestricted: %v", err)
	}
	// Liveness: the harness really dispatches (deployments pre-seeded in
	// team-a and team-b by newScopeTransitionEngine).
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 2)

	// Re-register with a ceiling admitting only a namespace that does not
	// exist: the recomputed effective set is empty and must fail closed.
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
		ScopeCeiling: ScopeCeiling{AllowedNamespaces: []string{"nonexistent"}},
	}); err != nil {
		t.Fatalf("register with ceiling: %v", err)
	}

	// The backfill re-dispatches the informer cache; every object must now
	// mismatch and any pre-ceiling items must be gone.
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 0)
	// Keep observing: late deliveries from the running informer must not
	// resurrect items either.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got := store.CountForTarget(target.Namespace, target.Name); got != 0 {
			t.Fatalf("item count = %d after fail-closed re-registration, want 0", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestEngineCeilingFreeRegistrationCollects pins FR-5: with no ceiling the
// selector/no-set fallback keeps its historical (permissive) semantics.
func TestEngineCeilingFreeRegistrationCollects(t *testing.T) {
	t.Parallel()

	engine, store, profile := newScopeTransitionEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// No explicit effective set, no ceiling: the recompute matches nothing
	// (the harness namespaces carry no labels), and the fallback must still
	// collect the pinned namespace via the metadata.name pin.
	target := scopeTransitionTarget("selector-target", "team-a")
	target.Spec.NamespaceSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{corev1.LabelMetadataName: "team-a"},
	}
	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{}); err != nil {
		t.Fatalf("register selector target: %v", err)
	}
	waitForTargetItems(t, engine, store, target.Namespace, target.Name, 1)
}
