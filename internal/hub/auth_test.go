// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func allowIngestSAR(action k8stesting.Action) (bool, runtime.Object, error) {
	review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
	spec := review.Spec
	switch {
	case spec.NonResourceAttributes != nil &&
		spec.NonResourceAttributes.Path == ingestReportsPath &&
		spec.NonResourceAttributes.Verb == "post":
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	case spec.ResourceAttributes != nil &&
		spec.ResourceAttributes.Group == "kollect.dev" &&
		spec.ResourceAttributes.Resource == "kollectremoteclusters" &&
		spec.ResourceAttributes.Verb == "create":
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	}

	return true, review, nil
}

func TestIngestAuthMiddlewareKubernetesSuccess(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User: authenticationv1.UserInfo{
				Username: "system:serviceaccount:spoke-a:kollect-spoke",
			},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", allowIngestSAR)

	called := false
	handler := IngestAuthConfig{
		Mode:   IngestAuthModeKubernetes,
		Client: client,
	}.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if !called {
		t.Fatal("handler not called")
	}
}

func TestIngestAuthMiddlewareMissingClusterID(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{Authenticated: true}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", allowIngestSAR)

	handler := IngestAuthConfig{
		Mode:   IngestAuthModeKubernetes,
		Client: client,
	}.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestIngestAuthMiddlewareForbiddenWithoutSAR(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{
			Authenticated: true,
			User: authenticationv1.UserInfo{
				Username: "system:serviceaccount:spoke-a:kollect-spoke",
			},
		}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

		return true, review, nil
	})

	handler := IngestAuthConfig{
		Mode:   IngestAuthModeKubernetes,
		Client: client,
	}.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizeIngestSARIncludesPlatformNamespace(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	var sawNamespace bool
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		if review.Spec.ResourceAttributes != nil &&
			review.Spec.ResourceAttributes.Resource == "kollectremoteclusters" {
			if review.Spec.ResourceAttributes.Namespace == "kollect-system" {
				sawNamespace = true
			}
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
		} else {
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}
		}

		return true, review, nil
	})

	cfg := IngestAuthConfig{
		Mode:              IngestAuthModeKubernetes,
		Client:            client,
		PlatformNamespace: "kollect-system",
	}
	ok, err := cfg.authorizeIngest(context.Background(), authenticationv1.UserInfo{
		Username: "system:serviceaccount:spoke-a:kollect-spoke",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !ok {
		t.Fatal("expected SAR with platform namespace to authorize ingest")
	}

	if !sawNamespace {
		t.Fatal("expected kollectremoteclusters SAR to include platform namespace")
	}
}

func TestAuthorizeIngestAllowsCreateOnRemoteCluster(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		spec := review.Spec
		if spec.NonResourceAttributes != nil {
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

			return true, review, nil
		}

		if spec.ResourceAttributes != nil &&
			spec.ResourceAttributes.Verb == "create" &&
			spec.ResourceAttributes.Resource == "kollectremoteclusters" {
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
		} else {
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}
		}

		return true, review, nil
	})

	cfg := IngestAuthConfig{Mode: IngestAuthModeKubernetes, Client: client}
	ok, err := cfg.authorizeIngest(context.Background(), authenticationv1.UserInfo{
		Username: "system:serviceaccount:spoke-a:kollect-spoke",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !ok {
		t.Fatal("expected create SAR on kollectremoteclusters to authorize ingest (ADR-0028)")
	}
}

func TestIngestAuthMiddlewareDisabledBypasses(t *testing.T) {
	t.Parallel()

	handler := IngestAuthConfig{Mode: IngestAuthModeDisabled}.Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
