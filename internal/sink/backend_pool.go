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

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

type poolKey string

type pooledEntry struct {
	backend  Backend
	specHash string
	lastUsed time.Time
}

// backendPoolTTL bounds the lifetime of an idle pooled backend entry.
// globalBackendPool is a process-lifetime map keyed by sink UID/namespace-
// name; EvictBackendPool/EvictBackendPoolByUID have no caller in this
// codebase today, so without this TTL the pool grows unbounded as sinks are
// created/deleted/renamed over the life of a long-running controller.
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
		mu      sync.Mutex
		entries map[poolKey]*pooledEntry
	}{
		entries: make(map[poolKey]*pooledEntry),
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

// ResetBackendPoolForTest evicts all pooled backends.
func ResetBackendPoolForTest() {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	for k, e := range globalBackendPool.entries {
		closeBackendLogged(e.backend, "pool reset")
		delete(globalBackendPool.entries, k)
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

	globalBackendPool.mu.Lock()
	pruneStaleEntriesLocked(now)
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

	backend, err := reg.NewBackend(spec, buildCtx)
	if err != nil {
		return nil, func() {}, err
	}

	globalBackendPool.mu.Lock()
	if old, ok := globalBackendPool.entries[key]; ok && old.specHash != specHash {
		closeBackendLogged(old.backend, "spec hash change")
	}

	globalBackendPool.entries[key] = &pooledEntry{
		backend:  backend,
		specHash: specHash,
		lastUsed: now,
	}
	globalBackendPool.mu.Unlock()

	return backend, func() {}, nil
}

// pruneStaleEntriesLocked evicts pooled backends that haven't been acquired
// in backendPoolTTL. Callers must hold globalBackendPool.mu. Triggered
// opportunistically on every acquireBackend call (AR-11) so the pool doesn't
// grow unbounded for long-lived controller processes as sinks churn —
// entries for deleted/renamed sinks stop being acquired and age out the next
// time any other key is acquired.
func pruneStaleEntriesLocked(now time.Time) {
	for k, entry := range globalBackendPool.entries {
		if entry == nil {
			delete(globalBackendPool.entries, k)
			continue
		}
		if now.Sub(entry.lastUsed) > backendPoolTTL {
			closeBackendLogged(entry.backend, "ttl expired")
			delete(globalBackendPool.entries, k)
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

// EvictBackendPool removes a cached backend by namespace/name (tests and sink spec updates).
func EvictBackendPool(namespace, name string) {
	if backendPoolDisabled.Load() {
		return
	}

	evictPoolKey(poolKeyForSink("", namespace, name))
}

// EvictBackendPoolByUID removes a cached backend keyed by sink object UID.
func EvictBackendPoolByUID(uid types.UID) {
	if backendPoolDisabled.Load() || uid == "" {
		return
	}

	evictPoolKey(poolKeyForSink(uid, "", ""))
}

func evictPoolKey(key poolKey) {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	if entry, ok := globalBackendPool.entries[key]; ok {
		closeBackendLogged(entry.backend, "explicit eviction")
		delete(globalBackendPool.entries, key)
	}
}
