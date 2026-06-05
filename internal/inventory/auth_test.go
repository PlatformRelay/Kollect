// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"fmt"
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

func TestAuthMiddlewareKubernetesSuccess(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User: authenticationv1.UserInfo{
				Username: "system:serviceaccount:team-a:portal",
				Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:team-a"},
			},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	called := false
	cfg := &AuthConfig{
		Mode:                AuthModeKubernetes,
		Client:              client,
		RequireInventoryGet: true,
	}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/inventory", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if !called {
		t.Fatal("handler not called")
	}
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	t.Parallel()

	cfg := &AuthConfig{
		Mode:   AuthModeKubernetes,
		Client: fake.NewSimpleClientset(), //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/inventory", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthMiddlewareDisabledBypasses(t *testing.T) {
	t.Parallel()

	cfg := &AuthConfig{Mode: AuthModeDisabled}
	handler := cfg.Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/inventory", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthenticateRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{Authenticated: false}

		return true, review, nil
	})

	authCfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client}
	if _, err := authCfg.authenticate(context.Background(), "bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthMiddlewareUsesListVerbOnIndex(t *testing.T) {
	t.Parallel()

	var gotVerb, gotNS string
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		if review.Spec.ResourceAttributes != nil {
			gotVerb = review.Spec.ResourceAttributes.Verb
			gotNS = review.Spec.ResourceAttributes.Namespace
		}
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	cfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client, RequireInventoryGet: true}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory?namespace=team-a", nil)
	req.Header.Set("Authorization", "Bearer tok")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotVerb != "list" || gotNS != "team-a" {
		t.Fatalf("verb=%q ns=%q", gotVerb, gotNS)
	}
}

func TestAuthMiddlewareUsesGetVerbOnNamedPath(t *testing.T) {
	t.Parallel()

	var gotVerb, gotNS, gotName string
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		if review.Spec.ResourceAttributes != nil {
			gotVerb = review.Spec.ResourceAttributes.Verb
			gotNS = review.Spec.ResourceAttributes.Namespace
			gotName = review.Spec.ResourceAttributes.Name
		}
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	cfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client, RequireInventoryGet: true}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory/team-a/my-inventory", nil)
	req.SetPathValue("namespace", "team-a")
	req.SetPathValue("name", "my-inventory")
	req.Header.Set("Authorization", "Bearer tok")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotVerb != "get" || gotNS != "team-a" || gotName != "my-inventory" {
		t.Fatalf("verb=%q ns=%q name=%q", gotVerb, gotNS, gotName)
	}
}

func TestAuthMiddlewareForbiddenWhenSARDenied(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

		return true, review, nil
	})

	cfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client, RequireInventoryGet: true}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAuthMiddlewareCacheHitForbidden(t *testing.T) {
	t.Parallel()

	sarCalls := 0
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sarCalls++
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

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
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	}

	if sarCalls != 1 {
		t.Fatalf("SAR calls = %d, want 1 (cached denial)", sarCalls)
	}
}

func TestBearerTokenErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header string
	}{
		{header: ""},
		{header: "Token abc"},
		{header: "Bearer "},
	}

	for _, tc := range cases {
		if _, err := bearerToken(tc.header); err == nil {
			t.Fatalf("header %q: expected error", tc.header)
		}
	}
}

func TestAuthCacheKeyIncludesNamespaceAndName(t *testing.T) {
	t.Parallel()

	if authCacheKey("h", "list", "a", "") == authCacheKey("h", "list", "b", "") {
		t.Fatal("namespace must affect cache key")
	}
}

func TestAuthMiddlewareSkipsSARWhenNotRequired(t *testing.T) {
	t.Parallel()

	sarCalls := 0
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sarCalls++

		return true, nil, nil
	})

	cfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client, RequireInventoryGet: false}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if sarCalls != 0 {
		t.Fatalf("SAR calls = %d, want 0", sarCalls)
	}
}

func TestAuthMiddlewareSARError(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("apiserver unavailable")
	})

	cfg := &AuthConfig{Mode: AuthModeKubernetes, Client: client, RequireInventoryGet: true}
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthDisabledModeNone(t *testing.T) {
	t.Parallel()

	cfg := AuthConfig{Mode: "none"}
	if !cfg.AuthDisabled() {
		t.Fatal("mode none should disable auth")
	}
}

func TestAuthMiddlewareCacheHitAllowed(t *testing.T) {
	t.Parallel()

	sarCalls := 0
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User:          authenticationv1.UserInfo{Username: "u"},
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
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
	}

	if sarCalls != 1 {
		t.Fatalf("SAR calls = %d, want 1", sarCalls)
	}
}
