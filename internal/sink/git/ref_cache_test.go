// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"testing"
	"time"
)

func TestRefCache_ttl(t *testing.T) {
	t.Parallel()

	cache := newRefCache(30 * time.Millisecond)
	key := "test-key"
	errSentinel := errors.New("probe failed")

	cache.set(key, errSentinel)
	if ok, got := cache.get(key); !ok || got != errSentinel {
		t.Fatalf("expected cached error, got %v ok=%v", got, ok)
	}

	time.Sleep(40 * time.Millisecond)
	if ok, _ := cache.get(key); ok {
		t.Fatal("expected cache expiry")
	}
}

func TestNewRefCache_nonPositiveTTLDefaults(t *testing.T) {
	t.Parallel()

	cache := newRefCache(0)
	if cache.ttl != defaultRefCacheTTL {
		t.Fatalf("ttl = %v, want default %v", cache.ttl, defaultRefCacheTTL)
	}
}

func TestRefCacheKey_stable(t *testing.T) {
	t.Parallel()

	a := refCacheKey("https://example.com/r.git", Auth{Username: "u", Token: "t"})
	b := refCacheKey("https://example.com/r.git", Auth{Username: "u", Token: "t"})
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
}
