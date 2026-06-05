// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

package hub

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const (
	// IngestAuthModeKubernetes validates bearer tokens via TokenReview + SAR (ADR-0028).
	IngestAuthModeKubernetes = "kubernetes"
	// IngestAuthModeDisabled disables ingest auth (local dev / CI only).
	IngestAuthModeDisabled = "disabled"
)

// IngestAuthConfig configures hub spoke-report HTTP authentication.
type IngestAuthConfig struct {
	Mode              string
	Client            kubernetes.Interface
	PlatformNamespace string
}

// AuthDisabled reports whether ingest auth middleware should be bypassed.
func (a IngestAuthConfig) AuthDisabled() bool {
	switch a.Mode {
	case IngestAuthModeDisabled, "none":
		return true
	default:
		return false
	}
}

// Middleware wraps handlers with Kubernetes token validation when enabled.
func (a IngestAuthConfig) Middleware(next http.Handler) http.Handler {
	if a.AuthDisabled() || a.Client == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)

			return
		}

		user, err := a.authenticate(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		ok, err := a.authorizeIngest(r.Context(), user)
		if err != nil {
			http.Error(w, "authorization check failed", http.StatusInternalServerError)

			return
		}

		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)

			return
		}

		if strings.TrimSpace(r.Header.Get(kollectdevv1alpha1.HeaderClusterID)) == "" {
			http.Error(w, "missing "+kollectdevv1alpha1.HeaderClusterID+" header", http.StatusBadRequest)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func bearerToken(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("expected Bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", fmt.Errorf("empty bearer token")
	}

	return token, nil
}

func (a IngestAuthConfig) authenticate(ctx context.Context, token string) (authenticationv1.UserInfo, error) {
	review := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := a.Client.AuthenticationV1().TokenReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return authenticationv1.UserInfo{}, fmt.Errorf("token review: %w", err)
	}

	if !result.Status.Authenticated {
		return authenticationv1.UserInfo{}, fmt.Errorf("token not authenticated")
	}

	return result.Status.User, nil
}

func (a IngestAuthConfig) authorizeIngest(ctx context.Context, user authenticationv1.UserInfo) (bool, error) {
	checks := []authorizationv1.SubjectAccessReviewSpec{
		{
			User:   user.Username,
			Groups: user.Groups,
			NonResourceAttributes: &authorizationv1.NonResourceAttributes{
				Path: ingestReportsPath,
				Verb: "post",
			},
		},
		{
			User:   user.Username,
			Groups: user.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: a.PlatformNamespace,
				Group:     "kollect.dev",
				Resource:  "kollectremoteclusters",
				Verb:      "create",
			},
		},
	}

	for _, spec := range checks {
		allowed, err := a.subjectAccessReview(ctx, spec)
		if err != nil {
			return false, err
		}

		if allowed {
			return true, nil
		}
	}

	return false, nil
}

func (a IngestAuthConfig) subjectAccessReview(
	ctx context.Context,
	spec authorizationv1.SubjectAccessReviewSpec,
) (bool, error) {
	review := &authorizationv1.SubjectAccessReview{Spec: spec}

	result, err := a.Client.AuthorizationV1().SubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("subject access review: %w", err)
	}

	return result.Status.Allowed, nil
}
