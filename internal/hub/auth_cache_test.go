// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
)

// EC-P1-10: ingest auth cache avoids repeated TokenReview/SAR within TTL.
func TestIngestAuthCache_hitMiss(t *testing.T) {
	t.Parallel()

	cache := newIngestAuthCache(30 * time.Second)
	user := authenticationv1.UserInfo{Username: "spoke"}

	cache.set("key", user, true)
	if allowed, ok := cache.getAllowed("key"); !ok || !allowed {
		t.Fatal("expected cache hit")
	}

	cache.set("deny", user, false)
	if allowed, ok := cache.getAllowed("deny"); !ok || allowed {
		t.Fatal("expected cached denial")
	}
}

func TestIngestAuthCache_expiresAfterTTL(t *testing.T) {
	t.Parallel()

	cache := newIngestAuthCache(20 * time.Millisecond)
	user := authenticationv1.UserInfo{Username: "spoke"}
	cache.set("key", user, true)

	time.Sleep(30 * time.Millisecond)

	if _, ok := cache.getAllowed("key"); ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestIngestAuthCache_nilSafe(t *testing.T) {
	t.Parallel()

	var cache *ingestAuthCache
	if allowed, ok := cache.getAllowed("key"); ok || allowed {
		t.Fatal("nil cache must miss")
	}
	cache.set("key", authenticationv1.UserInfo{}, true)
}
