// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=authorization.k8s.io,resources=selfsubjectaccessreviews,verbs=create

package collect

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"github.com/platformrelay/kollect/internal/metrics"
)

// defaultAccessCacheCapacity bounds the number of distinct {gvr, ns, verb}
// decisions retained (REL-05). Key cardinality is groupresource × namespace ×
// verb × version; a large cluster (hundreds of namespaces × tens of GVRs) can
// exceed 1024, so the cap leans high — the goal is bounding memory, not
// tightness. Each entry is tiny (bool + time + short string, well under 200 B),
// so 4096 entries stay under ~1 MiB while still preventing unbounded growth.
const defaultAccessCacheCapacity = 4096

// accessCacheEntry is one cached SAR decision. It carries its own key so the LRU
// list element can be evicted from the backing map without a reverse lookup.
type accessCacheEntry struct {
	key       string
	allowed   bool
	expiresAt time.Time
}

// AccessChecker caches SelfSubjectAccessReview results for the operator service
// account behind a bounded LRU (REL-05): reads and writes refresh recency, and
// once the map reaches capacity the least-recently-used key is evicted. Evicting
// a key only drops a cached decision — a later request for it issues a fresh SAR
// and returns the same result, so correctness is preserved.
type AccessChecker struct {
	client   kubernetes.Interface
	mu       sync.Mutex
	cacheTTL time.Duration
	capacity int
	// lru orders entries most-recently-used (front) to least (back).
	lru   *list.List
	cache map[string]*list.Element
}

// NewAccessChecker returns a checker backed by the Kubernetes authorization API.
func NewAccessChecker(client kubernetes.Interface) *AccessChecker {
	return &AccessChecker{
		client:   client,
		cacheTTL: 30 * time.Second,
		capacity: defaultAccessCacheCapacity,
		lru:      list.New(),
		cache:    make(map[string]*list.Element),
	}
}

func accessCacheKey(gvr schema.GroupVersionResource, namespace, verb string) string {
	return fmt.Sprintf("%s/%s/%s/%s", gvr.GroupResource().String(), namespace, verb, gvr.Version)
}

// CanAccess reports whether the operator may perform verb on gvr in namespace.
func (a *AccessChecker) CanAccess(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace, verb string,
) (bool, error) {
	if a == nil || a.client == nil {
		return true, nil
	}

	key := accessCacheKey(gvr, namespace, verb)

	if allowed, ok := a.lookup(key); ok {
		metrics.AccessCacheTotal.WithLabelValues(metrics.AccessCacheResultHit).Inc()

		return allowed, nil
	}
	metrics.AccessCacheTotal.WithLabelValues(metrics.AccessCacheResultMiss).Inc()

	attrs := &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      verb,
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
	}

	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attrs,
		},
	}

	result, err := a.client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("self subject access review: %w", err)
	}

	allowed := result.Status.Allowed

	a.store(key, allowed)

	return allowed, nil
}

// lookup returns a cached decision when present and unexpired, refreshing its
// recency. An expired entry is dropped and reported as a miss. Safe for
// concurrent use.
func (a *AccessChecker) lookup(key string) (allowed, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	el, present := a.cache[key]
	if !present {
		return false, false
	}

	entry, _ := el.Value.(*accessCacheEntry)
	if !time.Now().Before(entry.expiresAt) {
		a.lru.Remove(el)
		delete(a.cache, key)

		return false, false
	}

	a.lru.MoveToFront(el)

	return entry.allowed, true
}

// store records (or refreshes) a decision, evicting the least-recently-used key
// when the cache is at capacity. Safe for concurrent use.
func (a *AccessChecker) store(key string, allowed bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	expiresAt := time.Now().Add(a.cacheTTL)

	if el, ok := a.cache[key]; ok {
		entry, _ := el.Value.(*accessCacheEntry)
		entry.allowed = allowed
		entry.expiresAt = expiresAt
		a.lru.MoveToFront(el)

		return
	}

	el := a.lru.PushFront(&accessCacheEntry{key: key, allowed: allowed, expiresAt: expiresAt})
	a.cache[key] = el

	for a.capacity > 0 && a.lru.Len() > a.capacity {
		oldest := a.lru.Back()
		if oldest == nil {
			break
		}
		a.lru.Remove(oldest)
		if evicted, ok := oldest.Value.(*accessCacheEntry); ok {
			delete(a.cache, evicted.key)
		}
	}
}

// cacheLen reports the number of cached entries (for tests).
func (a *AccessChecker) cacheLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return len(a.cache)
}

// Invalidate clears cached decisions (for tests).
func (a *AccessChecker) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lru = list.New()
	a.cache = make(map[string]*list.Element)
}
