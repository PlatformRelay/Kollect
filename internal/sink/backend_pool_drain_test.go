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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

// Tests for SCR-3: a retired pooled backend drains (closed once, after its last holder releases),
// and the store fence keeps an older build from replacing or re-populating a newer pool state.

const drainWait = 5 * time.Second

var (
	errDrainExportOnClosed = errors.New("export ran on a closed backend")
	errDrainRejected       = errors.New("constructor rejected the credentials")
)

// drainBackend is a fake backend whose Export can block on a channel and fails if the backend
// was closed while it ran.
type drainBackend struct {
	closes atomic.Int32
	token  string
	block  chan struct{}
}

func (b *drainBackend) Type() string               { return "s3" }
func (b *drainBackend) Capabilities() Capabilities { return cap.SnapshotStore() }

func (b *drainBackend) Export(context.Context, []byte, string) error {
	if b.block != nil {
		<-b.block
	}
	if b.closes.Load() != 0 {
		return errDrainExportOnClosed
	}

	return nil
}

func (b *drainBackend) Close() error {
	b.closes.Add(1)

	return nil
}

// drainRegistry builds drainBackends, records them in call order, can hold a build in the
// factory (gate) and rejects the token "sentinel-token-bad".
type drainRegistry struct {
	*Registry

	mu       sync.Mutex
	backends []*drainBackend
	gate     func(call int)
}

func newDrainRegistry() *drainRegistry {
	dr := &drainRegistry{Registry: NewRegistry()}
	dr.Register("s3", func(_ kollectdevv1alpha1.KollectSinkSpec, bc BuildContext) (Backend, error) {
		token := string(bc.SecretData["token"])
		if token == "sentinel-token-bad" {
			return nil, errDrainRejected
		}
		b := &drainBackend{token: token}
		dr.mu.Lock()
		call := len(dr.backends)
		dr.backends = append(dr.backends, b)
		gate := dr.gate
		dr.mu.Unlock()
		if gate != nil {
			gate(call)
		}

		return b, nil
	})

	return dr
}

func (dr *drainRegistry) built() []*drainBackend {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	return append([]*drainBackend(nil), dr.backends...)
}

func (dr *drainRegistry) setGate(gate func(call int)) {
	dr.mu.Lock()
	dr.gate = gate
	dr.mu.Unlock()
}

func drainSpec() kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
}

func drainClient(t *testing.T, token string) client.Client {
	t.Helper()

	return rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", token)))
}

func rotateTo(t *testing.T, c client.Client, token string) {
	t.Helper()

	setSecretData(t, c, rotationSinkNS, "creds", tokenData("token", token), nil)
}

type acquireResult struct {
	backend Backend
	release func()
	err     error
}

// acquireAsync runs one acquire for sinkName in a goroutine.
func acquireAsync(
	c client.Client, reader client.Reader, reg *Registry, sinkName string, spec kollectdevv1alpha1.KollectSinkSpec,
) <-chan acquireResult {
	out := make(chan acquireResult, 1)
	go func() {
		b, release, err := acquireBackendWithReader(context.Background(), c, reader, reg, rotationSinkNS, sinkName,
			types.UID("uid-"+sinkName), spec)
		out <- acquireResult{backend: b, release: release, err: err}
	}()

	return out
}

func waitAcquire(t *testing.T, ch <-chan acquireResult) acquireResult {
	t.Helper()

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("acquire: %v", r.err)
		}

		return r
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for an acquire")
	}

	return acquireResult{}
}

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(drainWait):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func pooledBackendFor(sinkName string) Backend {
	if entry := pooledEntryFor(sinkName); entry != nil {
		return entry.backend
	}

	return nil
}

// holdFirstBuild gates the factory's first call: started fires when it is inside the factory, and
// the build returns when proceed is closed.
func holdFirstBuild(reg *drainRegistry) (started, proceed chan struct{}) {
	started = make(chan struct{})
	proceed = make(chan struct{})
	var once sync.Once
	reg.setGate(func(call int) {
		if call != 0 {
			return
		}
		once.Do(func() { close(started) })
		<-proceed
	})

	return started, proceed
}

// exportDuringRetire runs E1's Export on b1 in a goroutine while retire runs, then checks that b1
// is not closed until E1 releases and that E1's Export saw an open backend.
func exportDuringRetire(t *testing.T, b1 *drainBackend, release1 func(), retire func()) {
	t.Helper()

	b1.block = make(chan struct{})
	exported := make(chan error, 1)
	go func() { exported <- b1.Export(context.Background(), nil, "") }()

	retire()
	if closes := b1.closes.Load(); closes != 0 {
		t.Fatalf("retired backend closed %d times while E1 still holds it, want 0", closes)
	}

	close(b1.block)
	select {
	case err := <-exported:
		if err != nil {
			t.Fatalf("E1's export: %v", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for E1's export")
	}
	if closes := b1.closes.Load(); closes != 0 {
		t.Fatalf("retired backend closed %d times before E1 released it, want 0", closes)
	}

	release1()
	if closes := b1.closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after E1 released it, want 1", closes)
	}
}

func TestAcquireBackend_rotationDuringExportDefersClose(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "drain", drainSpec())
	exportDuringRetire(t, b1.(*drainBackend), release1, func() {
		rotateTo(t, c, "sentinel-token-B")
		b2, release2 := mustAcquire(t, c, c, reg.Registry, "drain", drainSpec())
		defer release2()
		if b2 == b1 || b2.(*drainBackend).token != "sentinel-token-B" {
			t.Fatal("E2 did not get a backend built from the rotated Secret")
		}
	})
}

func TestAcquireBackend_specChangeDuringExportDefersClose(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "drain", drainSpec())
	exportDuringRetire(t, b1.(*drainBackend), release1, func() {
		changed := drainSpec()
		changed.Endpoint = "changed"
		b2, release2 := mustAcquire(t, c, c, reg.Registry, "drain", changed)
		defer release2()
		if b2 == b1 {
			t.Fatal("E2 got the backend of the old spec")
		}
	})
}

func TestAcquireBackend_deleteEvictionDuringExportDefersClose(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "drain", drainSpec())
	exportDuringRetire(t, b1.(*drainBackend), release1, func() {
		EvictBackendPoolForSink("uid-drain", rotationSinkNS, "drain")
		if pooledEntryFor("drain") != nil {
			t.Fatal("delete eviction left the entry pooled")
		}
		globalBackendPool.mu.Lock()
		_, tombstoned := globalBackendPool.tombstones[poolKeyForSink("uid-drain", rotationSinkNS, "drain")]
		globalBackendPool.mu.Unlock()
		if !tombstoned {
			t.Fatal("delete eviction set no tombstone")
		}

		b2, release2 := mustAcquire(t, c, c, reg.Registry, "drain", drainSpec())
		if b2 == b1 || pooledEntryFor("drain") != nil {
			t.Fatal("the acquire after a delete eviction reused or pooled a backend")
		}
		release2()
		if closes := b2.(*drainBackend).closes.Load(); closes != 1 {
			t.Fatalf("unpooled backend after the eviction closed %d times on release, want 1", closes)
		}
	})
}

func TestAcquireBackend_ttlSkipsHeldEntry(t *testing.T) {
	enableRotationPool(t)
	t.Cleanup(func() { timeNow = time.Now })

	base := time.Now()
	timeNow = func() time.Time { return base }

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "held", drainSpec())

	later := base.Add(backendPoolTTL + time.Minute)
	timeNow = func() time.Time { return later }
	_, releaseOther := mustAcquire(t, c, c, reg.Registry, "other", drainSpec())
	releaseOther()

	held := b1.(*drainBackend)
	if closes := held.closes.Load(); closes != 0 {
		t.Fatalf("TTL prune closed a held backend %d times, want 0", closes)
	}
	if pooledBackendFor("held") != b1 {
		t.Fatal("TTL prune removed a held entry")
	}

	release1()
	if closes := held.closes.Load(); closes != 0 {
		t.Fatalf("release of a pooled, unretired entry closed it %d times, want 0", closes)
	}

	// Once released and still idle past the TTL, the next prune ages it out exactly once.
	_, releaseOther = mustAcquire(t, c, c, reg.Registry, "other", drainSpec())
	releaseOther()
	if closes := held.closes.Load(); closes != 1 {
		t.Fatalf("idle entry closed %d times by the prune after its release, want 1", closes)
	}
}

func TestAcquireBackend_slowBuilderDoesNotReplaceNewer(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "fence", drainSpec())
	waitSignal(t, started, "the slow build")

	rotateTo(t, c, "sentinel-token-6")
	b6, release6 := mustAcquire(t, c, c, reg.Registry, "fence", drainSpec())
	release6()

	close(proceed)
	g1 := waitAcquire(t, slow)

	if pooledBackendFor("fence") != b6 {
		t.Fatal("the older build replaced the newer pooled backend")
	}
	if closes := b6.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("newer pooled backend closed %d times, want 0", closes)
	}
	old := g1.backend.(*drainBackend)
	if old == b6 || old.token != "sentinel-token-5" {
		t.Fatal("the slow acquire did not get its own build")
	}
	if closes := old.closes.Load(); closes != 0 {
		t.Fatalf("slow build closed %d times before its release, want 0", closes)
	}
	g1.release()
	if closes := old.closes.Load(); closes != 1 {
		t.Fatalf("slow build closed %d times after its owning release, want 1", closes)
	}

	next, releaseNext := mustAcquire(t, c, c, reg.Registry, "fence", drainSpec())
	releaseNext()
	if next != b6 {
		t.Fatal("the next acquire did not get the newer pooled backend")
	}
}

func TestAcquireBackend_slowBuilderAfterRetireNotPooled(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "fence", drainSpec())
	waitSignal(t, started, "the slow build")

	deleteSecret(t, c, rotationSinkNS, "creds")
	if err := acquireErr(c, c, reg.Registry, "fence", drainSpec()); err == nil {
		t.Fatal("acquire succeeded with the Secret deleted")
	}

	close(proceed)
	g1 := waitAcquire(t, slow)

	if pooledEntryFor("fence") != nil {
		t.Fatal("a build that started before the retire was pooled")
	}
	old := g1.backend.(*drainBackend)
	g1.release()
	if closes := old.closes.Load(); closes != 1 {
		t.Fatalf("unpooled slow build closed %d times after its owning release, want 1", closes)
	}
}

func TestAcquireBackend_sameKeyBuildersShare(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	var inFactory sync.WaitGroup
	inFactory.Add(2)
	proceed := make(chan struct{})
	reg.setGate(func(int) {
		inFactory.Done()
		<-proceed
	})

	first := acquireAsync(c, c, reg.Registry, "share", drainSpec())
	second := acquireAsync(c, c, reg.Registry, "share", drainSpec())
	both := make(chan struct{})
	go func() { inFactory.Wait(); close(both) }()
	waitSignal(t, both, "both builders in the factory")
	reg.setGate(nil)
	close(proceed)

	r1, r2 := waitAcquire(t, first), waitAcquire(t, second)
	if r1.backend != r2.backend {
		t.Fatal("two builders at the same version did not share one backend")
	}
	if pooledBackendFor("share") != r1.backend {
		t.Fatal("the shared backend is not the pooled one")
	}

	built := reg.built()
	if len(built) != 2 {
		t.Fatalf("factory ran %d times, want 2", len(built))
	}
	kept := r1.backend.(*drainBackend)
	for _, b := range built {
		if b != kept && b.closes.Load() != 1 {
			t.Fatalf("surplus build closed %d times, want 1", b.closes.Load())
		}
	}

	// Both builders hold the shared entry: a retire (spec change) closes it only after both release.
	changed := drainSpec()
	changed.Endpoint = "changed"
	_, releaseChanged := mustAcquire(t, c, c, reg.Registry, "share", changed)
	releaseChanged()
	r1.release()
	if closes := kept.closes.Load(); closes != 0 {
		t.Fatalf("shared backend closed %d times while the second builder holds it, want 0", closes)
	}
	r2.release()
	if closes := kept.closes.Load(); closes != 1 {
		t.Fatalf("shared backend closed %d times after both released, want 1", closes)
	}
}

// barrierReader holds every metadata GET until n of them have returned.
type barrierReader struct {
	client.Reader
	arrived sync.WaitGroup
}

func (r *barrierReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := r.Reader.Get(ctx, key, obj, opts...)
	if _, ok := obj.(*metav1.PartialObjectMetadata); ok {
		r.arrived.Done()
		r.arrived.Wait()
	}

	return err
}

func TestAcquireBackend_concurrentAcquiresAfterRotation(t *testing.T) {
	enableRotationPool(t)

	const n = 16
	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "herd", drainSpec())
	release1()

	rotateTo(t, c, "sentinel-token-B")
	reader := &barrierReader{Reader: c}
	reader.arrived.Add(n)
	// Every acquire also waits in the factory until all n are there, so all n miss and build:
	// no store happens before the last lookup.
	var building sync.WaitGroup
	building.Add(n)
	reg.setGate(func(int) {
		building.Done()
		building.Wait()
	})

	results := make([]<-chan acquireResult, n)
	for i := range results {
		results[i] = acquireAsync(c, reader, reg.Registry, "herd", drainSpec())
	}
	got := make([]acquireResult, n)
	for i, ch := range results {
		got[i] = waitAcquire(t, ch)
	}

	shared := pooledBackendFor("herd")
	if shared == nil || shared == b1 || shared.(*drainBackend).token != "sentinel-token-B" {
		t.Fatal("no backend built from the rotated Secret is pooled")
	}
	for i, r := range got {
		if r.backend != shared {
			t.Fatalf("acquire %d did not get the pooled new backend", i)
		}
	}
	for _, r := range got {
		r.release()
		r.release()
	}

	if closes := b1.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times, want 1", closes)
	}
	if builds := len(reg.built()); builds != n+1 {
		t.Fatalf("factory ran %d times, want %d (the first build and one per concurrent acquire)", builds, n+1)
	}
	for _, b := range reg.built()[1:] {
		want := int32(1)
		if Backend(b) == shared {
			want = 0
		}
		if closes := b.closes.Load(); closes != want {
			t.Fatalf("backend %q closed %d times, want %d", b.token, closes, want)
		}
	}
}

func TestAcquireBackend_failedRebuildRetiresOld(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, release1 := mustAcquire(t, c, c, reg.Registry, "failed", drainSpec())
	old := b1.(*drainBackend)

	rotateTo(t, c, "sentinel-token-bad")
	if err := acquireErr(c, c, reg.Registry, "failed", drainSpec()); !errors.Is(err, errDrainRejected) {
		t.Fatalf("acquire with a rejected value: err %v, want the constructor's rejection", err)
	}
	if pooledEntryFor("failed") != nil {
		t.Fatal("a failed rebuild left the old backend pooled")
	}
	if closes := old.closes.Load(); closes != 0 {
		t.Fatalf("retired backend closed %d times while its holder runs, want 0", closes)
	}
	release1()
	if closes := old.closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after its holder released, want 1", closes)
	}

	rotateTo(t, c, "sentinel-token-B")
	b2, release2 := mustAcquire(t, c, c, reg.Registry, "failed", drainSpec())
	release2()
	if b2 == b1 || b2.(*drainBackend).token != "sentinel-token-B" {
		t.Fatal("the acquire after a failed rebuild returned the old backend")
	}
}

func TestRelease_idempotent(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b1, releaseA := mustAcquire(t, c, c, reg.Registry, "twice", drainSpec())
	_, releaseB := mustAcquire(t, c, c, reg.Registry, "twice", drainSpec())

	rotateTo(t, c, "sentinel-token-B")
	_, releaseNew := mustAcquire(t, c, c, reg.Registry, "twice", drainSpec())
	releaseNew()

	releaseA()
	releaseA()
	if closes := b1.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("a release called twice closed the retired backend (%d) while another holder runs", closes)
	}
	releaseB()
	if closes := b1.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after its last holder released, want 1", closes)
	}
	releaseB()
	if closes := b1.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("a repeated last release closed again (%d), want 1", closes)
	}
}

// staleMetaReader holds its metadata GET after the read until proceed is closed, so the acquire
// using it looks up the pool with metadata that predates a later rotation.
type staleMetaReader struct {
	client.Reader
	read    chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (r *staleMetaReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := r.Reader.Get(ctx, key, obj, opts...)
	if _, ok := obj.(*metav1.PartialObjectMetadata); ok {
		r.once.Do(func() {
			close(r.read)
			<-r.proceed
		})
	}

	return err
}

func TestAcquireBackend_staleReadDoesNotRetireNewer(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()

	b5, release5 := mustAcquire(t, c, c, reg.Registry, "stale", drainSpec())
	release5()

	stale := &staleMetaReader{Reader: c, read: make(chan struct{}), proceed: make(chan struct{})}
	late := acquireAsync(c, stale, reg.Registry, "stale", drainSpec())
	waitSignal(t, stale.read, "the stale metadata read")

	rotateTo(t, c, "sentinel-token-6")
	b6, release6 := mustAcquire(t, c, c, reg.Registry, "stale", drainSpec())
	if b6 == b5 {
		t.Fatal("the rotation did not rebuild")
	}

	close(stale.proceed)
	r := waitAcquire(t, late)

	if pooledBackendFor("stale") != b6 {
		t.Fatal("an acquire whose metadata read predates the rotation retired the newer pooled backend")
	}
	if r.backend.(*drainBackend).token != "sentinel-token-6" {
		t.Fatal("the late acquire did not build from the current Secret")
	}
	r.release()
	release6()
	if closes := b6.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("newer pooled backend closed %d times, want 0", closes)
	}
	if closes := b5.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("rotated-out backend closed %d times, want 1", closes)
	}
}

func TestAcquireBackend_staleNotFoundDoesNotRetireNewer(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()

	b5, release5 := mustAcquire(t, c, c, reg.Registry, "stale", drainSpec())
	release5()
	deleteSecret(t, c, rotationSinkNS, "creds")

	stale := &staleMetaReader{Reader: c, read: make(chan struct{}), proceed: make(chan struct{})}
	late := acquireAsync(c, stale, reg.Registry, "stale", drainSpec())
	waitSignal(t, stale.read, "the stale not-found read")

	if err := c.Create(context.Background(), rotationSecret(rotationSinkNS, "creds", "uid-creds-2",
		tokenData("token", "sentinel-token-6"))); err != nil {
		t.Fatalf("recreate secret: %v", err)
	}
	b6, release6 := mustAcquire(t, c, c, reg.Registry, "stale", drainSpec())
	defer release6()
	if b6 == b5 {
		t.Fatal("the recreated Secret did not rebuild")
	}

	// The late acquire's not-found read predates the entry, so it reads again, sees the recreated
	// Secret and shares the newer entry.
	close(stale.proceed)
	r := waitAcquire(t, late)
	defer r.release()

	if pooledBackendFor("stale") != b6 {
		t.Fatal("a not-found read that predates the recreate retired the newer pooled backend")
	}
	if r.backend != b6 {
		t.Fatal("the late acquire did not share the entry built from the recreated Secret")
	}
	if closes := b6.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("newer pooled backend closed %d times, want 0", closes)
	}
}

func TestAcquireBackend_observedChangeRetiresEntryStoredDuringRead(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	// X misses and builds from value 5; the Secret then moves to a value the constructor rejects.
	slow := acquireAsync(c, c, reg.Registry, "observed", drainSpec())
	waitSignal(t, started, "the slow build")
	rotateTo(t, c, "sentinel-token-bad")

	// A reads the rotated metadata; X stores its value-5 backend before A looks the pool up.
	reader := &staleMetaReader{Reader: c, read: make(chan struct{}), proceed: make(chan struct{})}
	observer := make(chan error, 1)
	go func() { observer <- acquireErr(c, reader, reg.Registry, "observed", drainSpec()) }()
	waitSignal(t, reader.read, "the observing metadata read")
	close(proceed)
	x := waitAcquire(t, slow)
	if pooledBackendFor("observed") != x.backend {
		t.Fatal("setup: the slow build was not pooled")
	}

	close(reader.proceed)
	select {
	case err := <-observer:
		if !errors.Is(err, errDrainRejected) {
			t.Fatalf("observing acquire: err %v, want the constructor's rejection", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for the observing acquire")
	}

	if pooledEntryFor("observed") != nil {
		t.Fatal("an acquire that observed the change left the older backend pooled after its rebuild failed")
	}
	x.release()
	if closes := x.backend.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after its holder released, want 1", closes)
	}
}

func TestResetBackendPoolForTest_lateReleaseDoesNotCloseAgain(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b, release := mustAcquire(t, c, c, reg.Registry, "reset", drainSpec())
	ResetBackendPoolForTest()
	release()
	if closes := b.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("backend closed %d times after a reset and a late release, want 1", closes)
	}
}

func TestEvictBackendPool_heldEntryDrains(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-A")
	reg := newDrainRegistry()

	b, release := mustAcquire(t, c, c, reg.Registry, "evict", drainSpec())
	EvictBackendPoolByUID("uid-evict")
	if pooledEntryFor("evict") != nil {
		t.Fatal("explicit eviction left the entry pooled")
	}
	if closes := b.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("explicit eviction closed a held backend %d times, want 0", closes)
	}
	release()
	if closes := b.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("evicted backend closed %d times after its release, want 1", closes)
	}
}

func TestAcquireBackend_agedGenerationDoesNotAdmitOldBuild(t *testing.T) {
	enableRotationPool(t)
	t.Cleanup(func() { timeNow = time.Now })

	base := time.Now()
	timeNow = func() time.Time { return base }

	c := drainClient(t, "sentinel-token-5")
	other := rotationClient(t, rotationSecret(rotationSinkNS, "other", "uid-other", tokenData("token", "sentinel-token-O")))
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	// X misses on a fresh key and stays in the factory longer than the TTL.
	slow := acquireAsync(c, c, reg.Registry, "aged", drainSpec())
	waitSignal(t, started, "the slow build")

	deleteSecret(t, c, rotationSinkNS, "creds")
	if err := acquireErr(c, c, reg.Registry, "aged", drainSpec()); err == nil {
		t.Fatal("acquire succeeded with the Secret deleted")
	}

	// Past the TTL, another sink's acquire runs the prune cycle that ages generations out.
	later := base.Add(backendPoolTTL + time.Minute)
	timeNow = func() time.Time { return later }
	otherSpec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "other"}}
	_, releaseOther := mustAcquire(t, other, other, reg.Registry, "other", otherSpec)
	releaseOther()

	close(proceed)
	x := waitAcquire(t, slow)
	if pooledEntryFor("aged") != nil {
		t.Fatal("a build older than a retire was pooled once the key's generation aged out")
	}
	x.release()
}

// rereadFailReader holds its first metadata GET after the read until proceed is closed, and fails
// every later metadata GET with err.
type rereadFailReader struct {
	client.Reader
	read    chan struct{}
	proceed chan struct{}
	err     error
	calls   atomic.Int32
}

func (r *rereadFailReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*metav1.PartialObjectMetadata); !ok {
		return r.Reader.Get(ctx, key, obj, opts...)
	}
	if r.calls.Add(1) > 1 {
		return r.err
	}
	err := r.Reader.Get(ctx, key, obj, opts...)
	close(r.read)
	<-r.proceed

	return err
}

func TestAcquireBackend_failedReReadKeepsObservedChange(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "reread", drainSpec())
	waitSignal(t, started, "the slow build")
	rotateTo(t, c, "sentinel-token-6")

	// A reads the rotated metadata; X stores its value-5 backend before A looks the pool up; A's
	// re-read then times out.
	reader := &rereadFailReader{Reader: c, read: make(chan struct{}), proceed: make(chan struct{}), err: context.DeadlineExceeded}
	observer := make(chan error, 1)
	go func() { observer <- acquireErr(c, reader, reg.Registry, "reread", drainSpec()) }()
	waitSignal(t, reader.read, "the observing metadata read")
	close(proceed)
	x := waitAcquire(t, slow)

	close(reader.proceed)
	select {
	case err := <-observer:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("observing acquire: err %v, want the re-read's timeout", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for the observing acquire")
	}

	if pooledEntryFor("reread") != nil {
		t.Fatal("a successful read observed the change, but a failed re-read left the older backend pooled")
	}
	x.release()
	if closes := x.backend.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after its holder released, want 1", closes)
	}
}
