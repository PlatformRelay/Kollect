// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type countingBackend struct {
	created atomic.Int32
}

func (c *countingBackend) Type() string               { return "counting" }
func (c *countingBackend) Capabilities() Capabilities { return SnapshotStoreCapabilities() }
func (c *countingBackend) Export(context.Context, []byte, string) error {
	return nil
}

// AR-11: globalBackendPool is a process-lifetime map keyed by sink
// UID/namespace-name. EvictBackendPool/EvictBackendPoolByUID exist but have
// no caller in this codebase today, so the pool currently has no eviction
// path at all and grows unbounded as sinks are created/deleted/renamed over
// the life of a long-running controller. A stale entry that is never
// acquired again must eventually be reclaimed; an entry still being used
// must survive.
func TestAcquireBackend_prunesStaleEntries(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() {
		timeNow = time.Now
		ResetBackendPoolForTest()
	})

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	base := time.Now()
	timeNow = func() time.Time { return base }

	_, releaseStale, err := acquireBackend(ctx, cl, reg, "team-a", "stale-sink", "", spec)
	if err != nil {
		t.Fatalf("stale acquire: %v", err)
	}
	releaseStale()

	activeBackend, releaseActive, err := acquireBackend(ctx, cl, reg, "team-a", "active-sink", "", spec)
	if err != nil {
		t.Fatalf("active acquire: %v", err)
	}
	releaseActive()

	// "active-sink" keeps being re-acquired at a cadence well inside the TTL
	// (simulating a live sink still exporting on its debounce interval), so
	// its lastUsed timestamp never goes stale; "stale-sink" is never touched
	// again (simulating its owning KollectSink being deleted).
	activeNow := base
	for i := 0; i < 3; i++ {
		activeNow = activeNow.Add(backendPoolTTL / 4)
		timeNow = func() time.Time { return activeNow }

		b, release, aerr := acquireBackend(ctx, cl, reg, "team-a", "active-sink", "", spec)
		if aerr != nil {
			t.Fatalf("active re-acquire %d: %v", i, aerr)
		}
		release()

		if b != activeBackend {
			t.Fatalf("active re-acquire %d returned a different backend instance", i)
		}
	}

	// Now jump far enough that "stale-sink" (last touched at base) is well
	// past backendPoolTTL, while "active-sink" (last touched at activeNow)
	// is still within it.
	farFuture := activeNow.Add(backendPoolTTL/4 + time.Minute)
	timeNow = func() time.Time { return farFuture }

	activeBackend2, releaseActive2, err := acquireBackend(ctx, cl, reg, "team-a", "active-sink", "", spec)
	if err != nil {
		t.Fatalf("active re-acquire: %v", err)
	}
	releaseActive2()

	if activeBackend2 != activeBackend {
		t.Fatal("active entry must survive the sweep and keep returning the same pooled instance")
	}

	globalBackendPool.mu.Lock()
	_, staleStillPresent := globalBackendPool.entries[poolKeyForSink("", "team-a", "stale-sink")]
	globalBackendPool.mu.Unlock()

	if staleStillPresent {
		t.Fatal("stale pooled backend should have been pruned after backendPoolTTL elapsed with no activity")
	}
}

func TestAcquireBackend_reusesPooledInstance(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	t.Cleanup(func() { EvictBackendPool("team-a", "pool") })

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b1 != b2 {
		t.Fatal("expected same pooled backend instance")
	}
}

func TestAcquireBackend_reusesPooledInstanceByUID(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	const uid = types.UID("pool-uid-abc")
	t.Cleanup(func() { EvictBackendPoolByUID(uid) })

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "renamed-a", uid, spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "renamed-b", uid, spec)
	if err != nil {
		t.Fatalf("second acquire after rename: %v", err)
	}
	release2()

	if b1 != b2 {
		t.Fatal("expected same pooled backend instance keyed by sink UID")
	}
}

func TestAcquireBackend_disabledPoolCreatesNewEachTime(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() {
		backendPoolDisabled.Store(false)
		ResetBackendPoolForTest()
	})

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b1 == b2 {
		t.Fatal("disabled pool must not reuse backend instances")
	}
}
