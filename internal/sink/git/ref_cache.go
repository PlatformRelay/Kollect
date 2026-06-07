// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const defaultRefCacheTTL = 30 * time.Second

type refCacheEntry struct {
	err       error
	expiresAt time.Time
}

type refCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]refCacheEntry
}

func newRefCache(ttl time.Duration) *refCache {
	if ttl <= 0 {
		ttl = defaultRefCacheTTL
	}

	return &refCache{ttl: ttl, entries: make(map[string]refCacheEntry)}
}

var lsRemoteRefCache = newRefCache(defaultRefCacheTTL)

func refCacheKey(endpoint string, auth Auth) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.TrimSpace(endpoint)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(auth.AuthType))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(auth.Username))
	_, _ = h.Write([]byte{0})
	if auth.Token != "" {
		_, _ = h.Write([]byte("token"))
	} else if auth.Password != "" {
		_, _ = h.Write([]byte("password"))
	} else if len(auth.SSHPrivateKey) > 0 {
		_, _ = h.Write([]byte("ssh"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (c *refCache) get(key string) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(c.entries, key)

		return nil, false
	}

	return entry.err, true
}

func (c *refCache) set(key string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = refCacheEntry{err: err, expiresAt: time.Now().Add(c.ttl)}
}
