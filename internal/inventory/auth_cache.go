// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
)

type authCacheEntry struct {
	user      authenticationv1.UserInfo
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

func (c *authCache) get(key string) (authenticationv1.UserInfo, bool, bool) {
	if c == nil {
		return authenticationv1.UserInfo{}, false, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return authenticationv1.UserInfo{}, false, false
	}

	return entry.user, entry.allowed, true
}

func (c *authCache) set(key string, user authenticationv1.UserInfo, allowed bool) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = authCacheEntry{
		user:      user,
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
