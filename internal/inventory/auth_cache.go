// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type authCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

type authCache struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]authCacheEntry
}

func newAuthCache(ttl time.Duration) *authCache {
	if ttl <= 0 {
		return nil
	}

	return &authCache{ttl: ttl, items: make(map[string]authCacheEntry)}
}

func (c *authCache) get(key string) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return false, false
	}

	return entry.allowed, true
}

func (c *authCache) set(key string, allowed bool) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = authCacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

func authCacheKey(tokenHash, verb, namespace, name string) string {
	return tokenHash + "|" + verb + "|" + namespace + "|" + name
}
