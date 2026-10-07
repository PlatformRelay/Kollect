// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

type countingBackend struct {
	created atomic.Int32
}

func (c *countingBackend) Type() string               { return "counting" }
func (c *countingBackend) Capabilities() Capabilities { return cap.SnapshotStore() }
func (c *countingBackend) Export(context.Context, []byte, string) error {
	return nil
}

type closeCountBackend struct {
	closes atomic.Int32
}

func (c *closeCountBackend) Type() string               { return "close-count" }
func (c *closeCountBackend) Capabilities() Capabilities { return cap.SnapshotStore() }
func (c *closeCountBackend) Export(context.Context, []byte, string) error {
	return nil
}
func (c *closeCountBackend) Close() error {
	c.closes.Add(1)

	return nil
}

type lockProbeBackend struct {
	closeErr error
	done     chan struct{}
}

func (b *lockProbeBackend) Type() string               { return "lock-probe" }
func (b *lockProbeBackend) Capabilities() Capabilities { return cap.SnapshotStore() }
func (b *lockProbeBackend) Export(context.Context, []byte, string) error {
	return nil
}

func (b *lockProbeBackend) Close() error {
	done := make(chan struct{})
	b.done = done

	go func() {
		globalBackendPool.mu.Lock()
		_ = len(globalBackendPool.entries)
		globalBackendPool.mu.Unlock()
		close(done)
	}()

	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-done:
		return nil
	case <-timer.C:
		b.closeErr = errors.New("close blocked on globalBackendPool.mu")

		return b.closeErr
	}
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

func TestAcquireBackend_sameSpecRaceClosesDuplicate(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "race-dup"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()

	var entered sync.WaitGroup
	entered.Add(2)

	var builtMu sync.Mutex
	var built []*closeCountBackend

	reg.Register("race-dup", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		b := &closeCountBackend{}

		builtMu.Lock()
		built = append(built, b)
		builtMu.Unlock()

		entered.Done()
		entered.Wait()

		return b, nil
	})

	ctx := context.Background()
	const (
		ns   = "team-a"
		name = "race-sink"
	)

	var wg sync.WaitGroup
	backends := make([]Backend, 2)
	errs := make([]error, 2)
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()

			b, release, err := acquireBackend(ctx, cl, reg, ns, name, "", spec)
			if release != nil {
				release()
			}

			backends[i] = b
			errs[i] = err
		}(i)
	}

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for both acquires")
	}

	for i, err := range errs {
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}

	builtMu.Lock()
	builtCopy := append([]*closeCountBackend(nil), built...)
	builtMu.Unlock()

	closedN := 0
	for _, b := range builtCopy {
		closedN += int(b.closes.Load())
	}

	same := backends[0] != nil && backends[0] == backends[1]
	if len(builtCopy) != 2 || !same || closedN != 1 {
		t.Fatalf("same-spec race: built=%d same=%t closes=%d", len(builtCopy), same, closedN)
	}

	kept, ok := backends[0].(*closeCountBackend)
	if !ok || kept.closes.Load() != 0 {
		t.Fatal("pooled backend should be the unclosed winner")
	}

	globalBackendPool.mu.Lock()
	entry := globalBackendPool.entries[poolKeyForSink("", ns, name)]
	var pooled Backend
	if entry != nil {
		pooled = entry.backend
	}
	globalBackendPool.mu.Unlock()

	if pooled != backends[0] {
		t.Fatal("pool entry backend should be the backend returned to both callers")
	}
}

func TestAcquireBackend_specChangeClosesOutsideLock(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()

	first := &lockProbeBackend{}
	second := &closeCountBackend{}

	reg.Register("lock-a", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return first, nil
	})
	reg.Register("lock-b", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return second, nil
	})

	ctx := context.Background()
	const (
		ns   = "team-a"
		name = "lock-sink"
	)

	specA := kollectdevv1alpha1.KollectSinkSpec{Type: "lock-a"}
	specB := kollectdevv1alpha1.KollectSinkSpec{Type: "lock-b"}

	got1, release1, err := acquireBackend(ctx, cl, reg, ns, name, "", specA)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	if got1 != first {
		t.Fatal("first acquire returned an unexpected backend")
	}

	got2, release2, err := acquireBackend(ctx, cl, reg, ns, name, "", specB)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if got2 != second {
		t.Fatal("second acquire returned an unexpected backend")
	}

	if first.done == nil {
		t.Fatal("spec change did not Close the replaced backend")
	}

	select {
	case <-first.done:
	case <-time.After(time.Second):
		t.Fatal("close probe did not finish")
	}

	if first.closeErr != nil {
		t.Fatalf("Close: %v", first.closeErr)
	}

	if second.closes.Load() != 0 {
		t.Fatal("replacement backend should stay open")
	}

	globalBackendPool.mu.Lock()
	entry := globalBackendPool.entries[poolKeyForSink("", ns, name)]
	var pooled Backend
	if entry != nil {
		pooled = entry.backend
	}
	globalBackendPool.mu.Unlock()

	if pooled != second {
		t.Fatal("pool should hold the replacement backend")
	}
}

func TestEvictBackendPool_closesOutsideLock(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	probed := &lockProbeBackend{}
	reg.Register("lock-evict", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return probed, nil
	})

	ctx := context.Background()
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "lock-evict"}
	if _, release, err := acquireBackend(ctx, cl, reg, "team-a", "evict-sink", "", spec); err != nil {
		t.Fatalf("acquire: %v", err)
	} else {
		release()
	}

	EvictBackendPool("team-a", "evict-sink")

	if probed.done == nil {
		t.Fatal("evict did not Close the pooled backend")
	}

	select {
	case <-probed.done:
	case <-time.After(time.Second):
		t.Fatal("evict close probe did not finish")
	}

	if probed.closeErr != nil {
		t.Fatalf("Close: %v", probed.closeErr)
	}

	globalBackendPool.mu.Lock()
	_, stillThere := globalBackendPool.entries[poolKeyForSink("", "team-a", "evict-sink")]
	globalBackendPool.mu.Unlock()

	if stillThere {
		t.Fatal("evict should drop the pool entry")
	}
}
