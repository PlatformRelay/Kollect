// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestAuthMiddlewareCacheHitsTokenReviewOnce(t *testing.T) {
	t.Parallel()

	tokenReviewCalls := 0
	sarCalls := 0

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		tokenReviewCalls++
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User: authenticationv1.UserInfo{
				Username: "system:serviceaccount:team-a:portal",
			},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sarCalls++
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	cfg := &AuthConfig{
		Mode:                AuthModeKubernetes,
		Client:              client,
		RequireInventoryGet: true,
		CacheTTL:            time.Minute,
	}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory", nil)
		req.Header.Set("Authorization", "Bearer good-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	}

	if tokenReviewCalls != 1 {
		t.Fatalf("tokenReview calls = %d, want 1 (cached second request)", tokenReviewCalls)
	}

	if sarCalls != 1 {
		t.Fatalf("SAR calls = %d, want 1 (cached second request)", sarCalls)
	}
}

func TestNewAuthCacheZeroTTL(t *testing.T) {
	t.Parallel()

	if c := newAuthCache(0); c != nil {
		t.Fatal("expected nil cache for non-positive TTL")
	}
}
