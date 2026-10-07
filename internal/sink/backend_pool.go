// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
)

type poolKey string

type pooledEntry struct {
	backend  Backend
	specHash string
	lastUsed time.Time
}

// backendPoolTTL bounds the lifetime of an idle pooled backend entry and of a
// delete-tombstone. globalBackendPool is a process-lifetime map keyed by sink
// UID/namespace-name; the family-sink controllers' delete watches evict a
// deleted sink's entry immediately (EvictBackendPoolForSink, BEP-1), so the
// TTL is the backstop that ages out entries and tombstones whose sink never
// produced a delete event (pipeline-mode one-shots, controller restarts) and
// keeps the pool bounded over the life of a long-running controller.
//
// This mirrors coalesceStateTTL's reasoning in internal/controller: it must
// stay comfortably above validation.MaxExportInterval (24h) so a slow-but-
// active sink's pooled backend is never pruned out from under it between
// export cycles, which would force an unnecessary reconnect/rebuild.
const backendPoolTTL = 2 * validation.MaxExportInterval

// timeNow is a test seam for backendPoolTTL pruning.
var timeNow = time.Now

var (
	backendPoolDisabled atomic.Bool

	globalBackendPool = struct {
		mu         sync.Mutex
		entries    map[poolKey]*pooledEntry
		tombstones map[poolKey]time.Time
	}{
		entries:    make(map[poolKey]*pooledEntry),
		tombstones: make(map[poolKey]time.Time),
	}
)

// DisableBackendPoolForTest turns off cross-export pooling (controller/envtest isolation).
func DisableBackendPoolForTest() {
	backendPoolDisabled.Store(true)
	ResetBackendPoolForTest()
}

// EnableBackendPoolForTest re-enables pooling after DisableBackendPoolForTest (test cleanup).
func EnableBackendPoolForTest() {
	backendPoolDisabled.Store(false)
}

// ResetBackendPoolForTest evicts all pooled backends and delete-tombstones.
func ResetBackendPoolForTest() {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	for k, e := range globalBackendPool.entries {
		closeBackendLogged(e.backend, "pool reset")
		delete(globalBackendPool.entries, k)
	}
	for k := range globalBackendPool.tombstones {
		delete(globalBackendPool.tombstones, k)
	}
}

func poolKeyForSink(sinkUID types.UID, sinkNamespace, sinkName string) poolKey {
	if sinkUID != "" {
		return poolKey("uid:" + string(sinkUID))
	}

	return poolKey("ns:" + sinkNamespace + "/" + sinkName)
}

func acquireBackend(
	ctx context.Context,
	c client.Client,
	reg *Registry,
	sinkNamespace, sinkName string,
	sinkUID types.UID,
	spec kollectdevv1alpha1.KollectSinkSpec,
) (Backend, func(), error) {
	if reg == nil {
		return nil, func() {}, fmt.Errorf("sink registry is not configured")
	}

	specHash, err := specFingerprint(spec)
	if err != nil {
		return nil, func() {}, err
	}

	if backendPoolDisabled.Load() {
		buildCtx, berr := BuildContextFromSpec(ctx, c, spec, sinkNamespace)
		if berr != nil {
			return nil, func() {}, berr
		}

		backend, berr := reg.NewBackend(spec, buildCtx)
		if berr != nil {
			return nil, func() {}, berr
		}

		return backend, func() { closeBackendLogged(backend, "pool disabled release") }, nil
	}

	key := poolKeyForSink(sinkUID, sinkNamespace, sinkName)
	now := timeNow()

	// Closed after Unlock: Pool.Close must not run while globalBackendPool.mu is held.
	var stale []Backend
	defer func() {
		closeBackendsLogged(stale, "ttl expired")
	}()

	globalBackendPool.mu.Lock()
	stale = pruneStaleEntriesLocked(now)
	if entry, ok := globalBackendPool.entries[key]; ok && entry.specHash == specHash {
		entry.lastUsed = now
		backend := entry.backend
		globalBackendPool.mu.Unlock()

		return backend, func() {}, nil
	}
	globalBackendPool.mu.Unlock()

	buildCtx, err := BuildContextFromSpec(ctx, c, spec, sinkNamespace)
	if err != nil {
		return nil, func() {}, err
	}

	built, err := reg.NewBackend(spec, buildCtx)
	if err != nil {
		return nil, func() {}, err
	}

	backend, discard, reason := storePooledBackend(key, specHash, now, built)
	if discard != nil {
		closeBackendLogged(discard, reason)
	}
	if reason == reasonDeleteTombstone {
		// Nothing was pooled for the deleted sink: the caller owns the built
		// backend and its release Closes it once the export is done.
		return backend, func() { closeBackendLogged(backend, "delete tombstone release") }, nil
	}

	return backend, func() {}, nil
}

// reasonDeleteTombstone marks the store decision for a tombstoned key: nothing
// was pooled, and acquireBackend hands the built backend to its caller with an
// owning release instead of closing it — the caller's release Closes it after
// its export is done, and the nats backend's closed latch (jetStream returns
// errBackendClosed after Close) means a Closed backend can no longer re-dial a
// fresh connection nothing would Close.
const reasonDeleteTombstone = "delete tombstone"

// storePooledBackend saves built under key, or keeps the pooled backend when
// specHash already matches. A key that carries a delete-tombstone (the sink
// was deleted while the build was in flight) pools nothing: the built backend
// is handed to the caller open, and the release acquireBackend returns Closes
// it exactly once (reasonDeleteTombstone). Nothing is ever pooled for the
// deleted sink (BEP-1). The function unlocks before returning so the caller
// can Close discard without holding globalBackendPool.mu.
func storePooledBackend(key poolKey, specHash string, now time.Time, built Backend) (Backend, Backend, string) {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	if _, deleted := globalBackendPool.tombstones[key]; deleted {
		return built, nil, reasonDeleteTombstone
	}

	old, ok := globalBackendPool.entries[key]
	if !ok {
		globalBackendPool.entries[key] = &pooledEntry{
			backend:  built,
			specHash: specHash,
			lastUsed: now,
		}

		return built, nil, ""
	}

	if old.specHash == specHash {
		old.lastUsed = now

		return old.backend, built, "duplicate pool backend"
	}

	globalBackendPool.entries[key] = &pooledEntry{
		backend:  built,
		specHash: specHash,
		lastUsed: now,
	}

	return built, old.backend, "spec hash change"
}

func closeBackendsLogged(backends []Backend, reason string) {
	for _, backend := range backends {
		closeBackendLogged(backend, reason)
	}
}

// pruneStaleEntriesLocked removes entries idle longer than backendPoolTTL and
// returns their backends, and ages out delete-tombstones on the same
// opportunistic cycle (a tombstone older than backendPoolTTL stops guarding
// its key). Caller holds globalBackendPool.mu and must Close those backends
// only after Unlock; Pool.Close under the mutex stalls every other acquire.
// Called on every acquire so deleted sinks age out (AR-11).
func pruneStaleEntriesLocked(now time.Time) []Backend {
	stale := make([]Backend, 0, len(globalBackendPool.entries))
	for k, entry := range globalBackendPool.entries {
		if entry == nil {
			delete(globalBackendPool.entries, k)
			continue
		}
		if now.Sub(entry.lastUsed) > backendPoolTTL {
			stale = append(stale, entry.backend)
			delete(globalBackendPool.entries, k)
		}
	}

	pruneExpiredTombstonesLocked(now)

	return stale
}

// pruneExpiredTombstonesLocked drops delete-tombstones older than
// backendPoolTTL (a tombstone older than the TTL stops guarding its key).
// Tombstones carry no backends, so no Close follows; caller holds
// globalBackendPool.mu. Run on the acquire path's opportunistic cycle and on
// the delete-eviction hook (evictPoolKeyForDelete) so a manager that never
// exports — its only pool touchpoint being delete evictions — still bounds
// the tombstone map instead of leaking one tombstone per delete for the life
// of the process (BEP-2).
func pruneExpiredTombstonesLocked(now time.Time) {
	for k, tombstoned := range globalBackendPool.tombstones {
		if now.Sub(tombstoned) > backendPoolTTL {
			delete(globalBackendPool.tombstones, k)
		}
	}
}

func specFingerprint(spec kollectdevv1alpha1.KollectSinkSpec) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("hash sink spec: %w", err)
	}

	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:]), nil
}

// EvictBackendPool removes a cached backend by namespace/name (test utility;
// production evictions go through EvictBackendPoolForSink).
func EvictBackendPool(namespace, name string) {
	if backendPoolDisabled.Load() {
		return
	}

	evictPoolKey(poolKeyForSink("", namespace, name))
}

// EvictBackendPoolByUID removes a cached backend keyed by sink object UID
// (test utility; production evictions go through EvictBackendPoolForSink).
func EvictBackendPoolByUID(uid types.UID) {
	if backendPoolDisabled.Load() || uid == "" {
		return
	}

	evictPoolKey(poolKeyForSink(uid, "", ""))
}

// EvictBackendPoolForSink is the delete-hook seam for the family-sink
// controllers (BEP-1): evicting a deleted sink's pooled backend by its object
// UID, falling back to the sink's namespace/name key only when the delete
// event's object carries no UID. Eviction is best-effort and idempotent: it
// never re-exports, retracts or reconciles, and deleting a sink with no pooled
// entry must not fail. It also records a delete-tombstone for the evicted key
// so an acquire-build still in flight when the delete landed is discarded
// instead of re-pooled for the deleted sink.
func EvictBackendPoolForSink(sinkUID types.UID, sinkNamespace, sinkName string) {
	if backendPoolDisabled.Load() {
		return
	}

	evictPoolKeyForDelete(poolKeyForSink(sinkUID, sinkNamespace, sinkName))
}

// evictPoolKeyForDelete drops the entry for key and records a delete-tombstone
// so storePooledBackend discards any backend built for the key afterwards.
// Only the delete hook records tombstones: TTL pruning, the acquire-time
// spec-hash swap and the EvictBackendPool/EvictBackendPoolByUID test helpers
// evict live entries and must never block a live sink's next build.
func evictPoolKeyForDelete(key poolKey) {
	now := timeNow()
	globalBackendPool.mu.Lock()
	entry, ok := globalBackendPool.entries[key]
	if ok {
		delete(globalBackendPool.entries, key)
	}
	globalBackendPool.tombstones[key] = now
	// Opportunistic sweep on the eviction path: a manager that only deletes
	// sinks (no exports, so acquireBackend's prune cycle never runs) ages its
	// tombstones out here instead of leaking one entry per delete. Tombstones
	// carry no backends, so the sweep is mutex-only.
	pruneExpiredTombstonesLocked(now)
	globalBackendPool.mu.Unlock()
	if ok {
		closeBackendLogged(entry.backend, "sink delete eviction")
	}
}

func evictPoolKey(key poolKey) {
	globalBackendPool.mu.Lock()
	entry, ok := globalBackendPool.entries[key]
	if ok {
		delete(globalBackendPool.entries, key)
	}
	globalBackendPool.mu.Unlock()
	if ok {
		closeBackendLogged(entry.backend, "explicit eviction")
	}
}
