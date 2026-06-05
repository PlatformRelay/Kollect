// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=authorization.k8s.io,resources=selfsubjectaccessreviews,verbs=create

package collect

import (
	"context"
	"fmt"
	"sync"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// AccessChecker caches SelfSubjectAccessReview results for the operator service account.
var accessSARCacheTTL = 30 * time.Second

type accessCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

// AccessChecker caches SelfSubjectAccessReview results for the operator service account.
type AccessChecker struct {
	client kubernetes.Interface
	mu     sync.RWMutex
	cache  map[string]accessCacheEntry
}

// NewAccessChecker returns a checker backed by the Kubernetes authorization API.
func NewAccessChecker(client kubernetes.Interface) *AccessChecker {
	return &AccessChecker{
		client: client,
		cache:  make(map[string]accessCacheEntry),
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

	a.mu.RLock()
	if entry, ok := a.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		a.mu.RUnlock()

		return entry.allowed, nil
	}
	a.mu.RUnlock()

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

	a.mu.Lock()
	a.cache[key] = accessCacheEntry{allowed: allowed, expiresAt: time.Now().Add(accessSARCacheTTL)}
	a.mu.Unlock()

	return allowed, nil
}

// Invalidate clears cached decisions (for tests).
func (a *AccessChecker) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cache = make(map[string]accessCacheEntry)
}
