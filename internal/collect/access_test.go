// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

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

	orig := accessSARCacheTTL
	accessSARCacheTTL = 25 * time.Millisecond
	t.Cleanup(func() { accessSARCacheTTL = orig })

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
