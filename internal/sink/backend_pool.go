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

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type poolKey struct {
	namespace string
	name      string
}

type pooledEntry struct {
	backend  Backend
	specHash string
}

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

// ResetBackendPoolForTest evicts all pooled backends.
func ResetBackendPoolForTest() {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	for k, e := range globalBackendPool.entries {
		_ = closeBackend(e.backend)
		delete(globalBackendPool.entries, k)
	}
}

func acquireBackend(
	ctx context.Context,
	c client.Client,
	reg *Registry,
	sinkNamespace, sinkName string,
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

		return backend, func() { _ = closeBackend(backend) }, nil
	}

	key := poolKey{namespace: sinkNamespace, name: sinkName}

	globalBackendPool.mu.Lock()
	if entry, ok := globalBackendPool.entries[key]; ok && entry.specHash == specHash {
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
		_ = closeBackend(old.backend)
	}

	globalBackendPool.entries[key] = &pooledEntry{
		backend:  backend,
		specHash: specHash,
	}
	globalBackendPool.mu.Unlock()

	return backend, func() {}, nil
}

func specFingerprint(spec kollectdevv1alpha1.KollectSinkSpec) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("hash sink spec: %w", err)
	}

	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:]), nil
}

// EvictBackendPool removes a cached backend (tests and sink spec updates).
func EvictBackendPool(namespace, name string) {
	if backendPoolDisabled.Load() {
		return
	}

	key := poolKey{namespace: namespace, name: name}

	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	if entry, ok := globalBackendPool.entries[key]; ok {
		_ = closeBackend(entry.backend)
		delete(globalBackendPool.entries, key)
	}
}
