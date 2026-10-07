// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// Tests for the delete-hook seam (BEP-1, BEP-2): EvictBackendPoolForSink is
// the hook the family-sink controllers' delete watches call, and it now
// evicts: the UID-keyed entry is gone and the pooled backend's Close ran, an
// in-flight acquire-build at eviction is discarded rather than re-pooled
// (delete-tombstone), an empty-UID delete falls back to the namespace/name
// key, a delete without a pooled entry is a no-op, and a spec-update is not
// an eviction.

func TestEvictBackendPoolForSink_evictsEntryAndCloses(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	pooled := &closeCountBackend{}
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return pooled, nil
	})

	const (
		uid  = types.UID("evict-uid-1")
		ns   = "team-a"
		name = "evict-sink"
	)

	b, release, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, spec)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release()

	if b != pooled {
		t.Fatal("acquire returned an unexpected backend")
	}

	EvictBackendPoolForSink(uid, ns, name)

	globalBackendPool.mu.Lock()
	_, stillThere := globalBackendPool.entries[poolKeyForSink(uid, "", "")]
	globalBackendPool.mu.Unlock()

	if stillThere {
		t.Fatal("delete-hook seam left the pooled entry in place (want gone)")
	}

	if pooled.closes.Load() != 1 {
		t.Fatalf("delete-hook seam did not Close the pooled backend (closes=%d, want 1)", pooled.closes.Load())
	}
}

func TestEvictBackendPoolForSink_inFlightBuildDiscardedNotRepooled(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	built := &closeCountBackend{}
	reg := NewRegistry()

	entered := make(chan struct{})
	releaseBuild := make(chan struct{})
	var once sync.Once
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		once.Do(func() { close(entered) })
		<-releaseBuild

		return built, nil
	})

	const (
		uid  = types.UID("evict-uid-2")
		ns   = "team-a"
		name = "in-flight-sink"
	)

	acquired := make(chan Backend, 1)
	acquireErr := make(chan error, 1)
	go func() {
		b, release, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, spec)
		if release != nil {
			release()
		}

		acquired <- b
		acquireErr <- err
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the acquire-build to start")
	}

	// The build is in flight with nothing pooled yet: the delete lands here.
	EvictBackendPoolForSink(uid, ns, name)
	close(releaseBuild)

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the acquire to finish")
	}

	if err := <-acquireErr; err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// acquireBackend returned, so its store attempt has landed; a tombstoned
	// key must have discarded the built backend instead of pooling it.
	globalBackendPool.mu.Lock()
	_, repooled := globalBackendPool.entries[poolKeyForSink(uid, "", "")]
	globalBackendPool.mu.Unlock()

	if repooled {
		t.Fatal("in-flight build was re-pooled after the delete (want discarded, not pooled)")
	}

	if built.closes.Load() != 1 {
		t.Fatalf("discarded in-flight build was not Closed (closes=%d, want 1)", built.closes.Load())
	}
}

func TestEvictBackendPoolForSink_noUIDFallsBackToNamespaceName(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	pooled := &closeCountBackend{}
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return pooled, nil
	})

	// A delete event whose object carries no UID (DeleteStateUnknown tombstone)
	// must fall back to the sink's namespace/name key (BEP-1): pool under that
	// key first (the acquire path keys ns/name exactly when the UID is empty).
	const (
		ns   = "team-a"
		name = "fallback-sink"
	)

	b, release, err := acquireBackend(context.Background(), cl, reg, ns, name, "", spec)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release()

	if b != pooled {
		t.Fatal("acquire returned an unexpected backend")
	}

	EvictBackendPoolForSink("", ns, name)

	globalBackendPool.mu.Lock()
	_, stillThere := globalBackendPool.entries[poolKeyForSink("", ns, name)]
	globalBackendPool.mu.Unlock()

	if stillThere {
		t.Fatal("empty-UID fallback did not evict the namespace/name-keyed entry (want gone)")
	}

	if pooled.closes.Load() != 1 {
		t.Fatalf("empty-UID fallback did not Close the pooled backend (closes=%d, want 1)", pooled.closes.Load())
	}
}

func TestEvictBackendPoolForSink_noPooledEntryIsNoOp(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	unrelated := &closeCountBackend{}
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return unrelated, nil
	})

	// Seed one live entry for another sink: evicting an absent key must not
	// touch it (BEP-1 eviction is best-effort and idempotent, with no
	// collateral damage).
	const otherUID = types.UID("evict-uid-other")
	b, release, err := acquireBackend(context.Background(), cl, reg, "team-b", "live-sink", otherUID, spec)
	if err != nil {
		t.Fatalf("acquire for the unrelated sink: %v", err)
	}
	release()

	if b != unrelated {
		t.Fatal("acquire returned an unexpected backend")
	}

	globalBackendPool.mu.Lock()
	before := len(globalBackendPool.entries)
	globalBackendPool.mu.Unlock()

	EvictBackendPoolForSink(types.UID("evict-uid-absent"), "team-a", "never-pooled")

	globalBackendPool.mu.Lock()
	after := len(globalBackendPool.entries)
	_, unrelatedStillPooled := globalBackendPool.entries[poolKeyForSink(otherUID, "", "")]
	globalBackendPool.mu.Unlock()

	if after != before {
		t.Fatalf("evicting a sink without a pooled entry changed the pool (%d -> %d)", before, after)
	}

	if !unrelatedStillPooled {
		t.Fatal("evicting an absent key dropped an unrelated sink's pooled entry")
	}

	if unrelated.closes.Load() != 0 {
		t.Fatalf("evicting an absent key Closed an unrelated sink's backend (closes=%d)", unrelated.closes.Load())
	}
}

func TestEvictBackendPoolForSink_noNewAcquireRebuildsEvictedEntry(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	v1 := &closeCountBackend{}
	v2 := &closeCountBackend{}
	var builtCount atomic.Int32
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		if builtCount.Add(1) == 1 {
			return v1, nil
		}

		return v2, nil
	})

	const (
		uid  = types.UID("evict-uid-4")
		ns   = "team-a"
		name = "no-rebuild-sink"
	)

	if _, release, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, spec); err != nil {
		t.Fatalf("first acquire: %v", err)
	} else {
		release()
	}

	EvictBackendPoolForSink(uid, ns, name)

	// The sink no longer exists: a new acquire for the same UID must not
	// rebuild a pooled entry for it (BEP-1).
	if _, release, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, spec); err != nil {
		t.Fatalf("acquire after eviction: %v", err)
	} else {
		release()
	}

	globalBackendPool.mu.Lock()
	_, rebuilt := globalBackendPool.entries[poolKeyForSink(uid, "", "")]
	globalBackendPool.mu.Unlock()

	if rebuilt {
		t.Fatal("acquire after eviction re-pooled an entry for the deleted sink (want none)")
	}

	// D4's tombstone discards at store time, so the post-eviction build may
	// have run: if it did, the built backend must have been discarded (Closed),
	// never leaked open.
	if builtCount.Load() > 1 {
		if v2.closes.Load() != 1 {
			t.Fatalf("post-eviction build was not discarded (closes=%d, want 1)", v2.closes.Load())
		}
	}
}

func TestEvictBackendPoolForSink_specUpdateIsNotEviction(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	v1 := &closeCountBackend{}
	v2 := &closeCountBackend{}
	var builtCount atomic.Int32
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		if builtCount.Add(1) == 1 {
			return v1, nil
		}

		return v2, nil
	})

	const (
		uid  = types.UID("evict-uid-3")
		ns   = "team-a"
		name = "spec-update-sink"
	)

	specV1 := kollectdevv1alpha1.KollectSinkSpec{Type: "counting", Endpoint: "endpoint-v1"}
	specV2 := kollectdevv1alpha1.KollectSinkSpec{Type: "counting", Endpoint: "endpoint-v2"}

	b1, release1, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, specV1)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	if b1 != v1 {
		t.Fatal("first acquire returned an unexpected backend")
	}

	// A spec update is an Update event, not a delete: no delete-hook call
	// happens. The next acquire swaps the entry via the spec hash.
	b2, release2, err := acquireBackend(context.Background(), cl, reg, ns, name, uid, specV2)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b2 != v2 {
		t.Fatal("second acquire returned an unexpected backend")
	}

	globalBackendPool.mu.Lock()
	entry := globalBackendPool.entries[poolKeyForSink(uid, "", "")]
	var pooled Backend
	if entry != nil {
		pooled = entry.backend
	}
	globalBackendPool.mu.Unlock()

	if pooled != v2 {
		t.Fatal("spec update evicted the UID-keyed entry instead of swapping it on acquire")
	}

	if v1.closes.Load() != 1 {
		t.Fatalf("replaced backend was not Closed by the spec-hash swap (closes=%d, want 1)", v1.closes.Load())
	}

	if v2.closes.Load() != 0 {
		t.Fatalf("replacement backend should stay open (closes=%d)", v2.closes.Load())
	}
}
