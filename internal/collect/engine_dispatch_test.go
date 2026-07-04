// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

// newBlockedEngine builds an Engine with a full, undrained dispatch queue:
// dispatchOnce is pre-fired as a no-op so dispatch() never spins up workers,
// letting tests observe queue-full behavior deterministically.
func newBlockedEngine(t *testing.T) *Engine {
	t.Helper()

	e := &Engine{
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
		dispatchCh:   make(chan dispatchJob, 1),
	}
	e.dispatchOnce.Do(func() {})

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	e.dispatchCh <- dispatchJob{ctx: context.Background(), gvr: gvr, obj: nil, deleted: false}

	return e
}

func TestDispatch_BlocksOnFullQueueUntilContextCanceled(t *testing.T) {
	// Not t.Parallel(): asserts an exact delta on the package-global
	// CollectDispatchBackpressureTotal counter, which TestDispatch_BlocksOnFullQueueUntilSlotFrees
	// also increments.
	e := newBlockedEngine(t)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	before := testutil.ToFloat64(metrics.CollectDispatchBackpressureTotal)

	const ctxTimeout = 50 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		e.dispatch(ctx, gvr, nil, false)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not return after context cancellation; it must not process synchronously")
	}
	elapsed := time.Since(start)

	// A correct backpressure implementation blocks on the full queue until ctx
	// is canceled, so it must take roughly the full ctx timeout to return. If
	// it returns much sooner, it took the old synchronous-fallback shortcut
	// instead of genuinely blocking.
	if elapsed < ctxTimeout/2 {
		t.Fatalf("dispatch returned after %v, want >= ~%v (must block on full queue, not process synchronously)", elapsed, ctxTimeout)
	}

	if got := len(e.dispatchCh); got != 1 {
		t.Fatalf("dispatchCh len = %d, want 1 (job must not be force-processed inline)", got)
	}

	if after := testutil.ToFloat64(metrics.CollectDispatchBackpressureTotal); after != before+1 {
		t.Fatalf("CollectDispatchBackpressureTotal = %v, want %v", after, before+1)
	}
}

func TestDispatch_BlocksOnFullQueueUntilSlotFrees(t *testing.T) {
	// Not t.Parallel(): see note on TestDispatch_BlocksOnFullQueueUntilContextCanceled.
	e := newBlockedEngine(t)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	done := make(chan struct{})
	go func() {
		e.dispatch(context.Background(), gvr, nil, false)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("dispatch returned before queue had capacity")
	case <-time.After(50 * time.Millisecond):
	}

	<-e.dispatchCh // free a slot

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not enqueue once a slot freed")
	}

	if got := len(e.dispatchCh); got != 1 {
		t.Fatalf("dispatchCh len = %d, want 1 (second job enqueued)", got)
	}
}

func TestEngineTargetsByGVRIndex(t *testing.T) {
	t.Parallel()

	e := &Engine{
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
	}

	gvrApps := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvrCore := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	e.mu.Lock()
	e.targets["team-a/deploys"] = targetState{
		profile: kollectdevv1alpha1.KollectProfile{
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
	}
	e.indexTargetLocked("team-a/deploys", gvrApps)
	e.targets["team-b/configs"] = targetState{
		profile: kollectdevv1alpha1.KollectProfile{
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
		},
	}
	e.indexTargetLocked("team-b/configs", gvrCore)
	e.mu.Unlock()

	if got := len(e.targetsByGVR[gvrApps]); got != 1 {
		t.Fatalf("apps index len = %d, want 1", got)
	}

	e.mu.Lock()
	e.unindexTargetLocked("team-a/deploys", gvrApps)
	e.mu.Unlock()

	if got := len(e.targetsByGVR[gvrApps]); got != 0 {
		t.Fatalf("apps index after remove = %d, want 0", got)
	}
	if got := len(e.targetsByGVR[gvrCore]); got != 1 {
		t.Fatalf("core index len = %d, want 1", got)
	}
}
