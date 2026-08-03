// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/platformrelay/kollect/internal/metrics"
)

// allowAllSARClient returns a fake clientset whose SAR reactor allows everything
// and increments *calls once per SubjectAccessReview. The counter pointer lets a
// test assert how many times the underlying access check was actually invoked
// (i.e. how many cache misses reached the API server).
func allowAllSARClient(calls *int, mu *sync.Mutex) *fake.Clientset {
	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for SAR unit test
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			if calls != nil {
				mu.Lock()
				*calls++
				mu.Unlock()
			}
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

			return true, review, nil
		})

	return client
}

func TestAccessCheckerEvictsLRU(t *testing.T) {
	t.Parallel()

	calls := 0
	var mu sync.Mutex
	checker := NewAccessChecker(allowAllSARClient(&calls, &mu))
	checker.capacity = 2

	res := func(r string) schema.GroupVersionResource {
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: r}
	}
	must := func(gvr schema.GroupVersionResource) {
		if _, err := checker.CanAccess(context.Background(), gvr, "default", "list"); err != nil {
			t.Fatalf("CanAccess(%s): %v", gvr.Resource, err)
		}
	}

	must(res("a")) // miss -> calls=1
	must(res("b")) // miss -> calls=2
	must(res("c")) // miss -> calls=3, evicts "a" (oldest)

	if calls != 3 {
		t.Fatalf("expected 3 SAR calls after filling past cap, got %d", calls)
	}
	if got := checker.cacheLen(); got > checker.capacity {
		t.Fatalf("cache size %d exceeds cap %d", got, checker.capacity)
	}

	// "c" is the newest key -> still cached -> hit, no new SAR.
	must(res("c"))
	if calls != 3 {
		t.Fatalf("newest key should be retained (cached), got %d SAR calls", calls)
	}

	// "a" was evicted -> a fresh SubjectAccessReview must be issued.
	must(res("a"))
	if calls != 4 {
		t.Fatalf("evicted key should trigger a fresh SAR, got %d SAR calls", calls)
	}
	if got := checker.cacheLen(); got > checker.capacity {
		t.Fatalf("cache size %d exceeds cap %d after re-fetch", got, checker.capacity)
	}
}

func TestAccessCheckerConcurrentRespectsCap(t *testing.T) {
	t.Parallel()

	checker := NewAccessChecker(allowAllSARClient(nil, nil))
	checker.capacity = 8

	var wg sync.WaitGroup
	for g := range 50 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for j := range 20 {
				r := fmt.Sprintf("r%d", (g*20+j)%100)
				gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: r}
				_, _ = checker.CanAccess(context.Background(), gvr, "default", "list")
			}
		}(g)
	}
	wg.Wait()

	if got := checker.cacheLen(); got > checker.capacity {
		t.Fatalf("concurrent access grew cache to %d, exceeding cap %d", got, checker.capacity)
	}
}

func TestAccessCheckerRecordsHitMissMetrics(t *testing.T) {
	// NOT parallel: reads a global counter delta. Sequential tests run to
	// completion before any t.Parallel() test resumes, so no engine test can
	// perturb kollect_access_cache_total during this window.
	hit := metrics.AccessCacheTotal.WithLabelValues(metrics.AccessCacheResultHit)
	miss := metrics.AccessCacheTotal.WithLabelValues(metrics.AccessCacheResultMiss)
	beforeHit := testutil.ToFloat64(hit)
	beforeMiss := testutil.ToFloat64(miss)

	calls := 0
	var mu sync.Mutex
	checker := NewAccessChecker(allowAllSARClient(&calls, &mu))
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "configmaps"}

	if _, err := checker.CanAccess(context.Background(), gvr, "kube-system", "get"); err != nil {
		t.Fatalf("first CanAccess: %v", err) // miss
	}
	if _, err := checker.CanAccess(context.Background(), gvr, "kube-system", "get"); err != nil {
		t.Fatalf("second CanAccess: %v", err) // hit
	}

	if got := testutil.ToFloat64(miss) - beforeMiss; got != 1 {
		t.Fatalf("expected miss counter +1, got +%v", got)
	}
	if got := testutil.ToFloat64(hit) - beforeHit; got != 1 {
		t.Fatalf("expected hit counter +1, got +%v", got)
	}
}

func TestAccessCheckerCachesAllowed(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for SAR unit test
	calls := 0
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			calls++
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

			return true, review, nil
		})

	checker := NewAccessChecker(client)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	ok, err := checker.CanAccess(context.Background(), gvr, "default", "list")
	if err != nil || !ok {
		t.Fatalf("first check: ok=%v err=%v", ok, err)
	}

	ok, err = checker.CanAccess(context.Background(), gvr, "default", "list")
	if err != nil || !ok {
		t.Fatalf("second check: ok=%v err=%v", ok, err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 SAR call, got %d", calls)
	}

	checker.Invalidate()
	ok, err = checker.CanAccess(context.Background(), gvr, "default", "list")
	if err != nil || !ok {
		t.Fatalf("after invalidate: ok=%v err=%v", ok, err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 SAR calls after invalidate, got %d", calls)
	}
}

func TestAccessCheckerCacheExpiresAfterTTL(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck
	calls := 0
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			calls++
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

			return true, review, nil
		})

	checker := NewAccessChecker(client)
	checker.cacheTTL = 25 * time.Millisecond
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	_, _ = checker.CanAccess(context.Background(), gvr, "default", "list")
	time.Sleep(40 * time.Millisecond)
	_, _ = checker.CanAccess(context.Background(), gvr, "default", "list")

	if calls != 2 {
		t.Fatalf("expected 2 SAR calls after TTL expiry, got %d", calls)
	}
}

func TestAccessCheckerNilClientAllows(t *testing.T) {
	t.Parallel()

	var checker *AccessChecker
	ok, err := checker.CanAccess(context.Background(), schema.GroupVersionResource{}, "default", "list")
	if err != nil || !ok {
		t.Fatalf("nil checker should allow: ok=%v err=%v", ok, err)
	}

	ok, err = NewAccessChecker(nil).CanAccess(context.Background(), schema.GroupVersionResource{}, "default", "list")
	if err != nil || !ok {
		t.Fatalf("nil client should allow: ok=%v err=%v", ok, err)
	}
}

func TestAccessCheckerDeniedAndAPIError(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

			return true, review, nil
		},
	)

	checker := NewAccessChecker(client)
	ok, err := checker.CanAccess(
		context.Background(),
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		"default",
		"list",
	)
	if err != nil || ok {
		t.Fatalf("denied SAR: ok=%v err=%v", ok, err)
	}
}

func TestAccessCheckerAPIError(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("apiserver unavailable")
		},
	)

	checker := NewAccessChecker(client)
	_, err := checker.CanAccess(
		context.Background(),
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		"default",
		"list",
	)
	if err == nil {
		t.Fatal("expected SAR API error")
	}
}
