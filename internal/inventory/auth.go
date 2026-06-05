// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

package inventory

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// AuthModeKubernetes validates bearer tokens via TokenReview + SubjectAccessReview.
	AuthModeKubernetes = "kubernetes"
	// AuthModeDisabled disables auth (local dev / CI only).
	AuthModeDisabled = "disabled"
)

// AuthConfig configures inventory HTTP authentication (ADR-0404).
type AuthConfig struct {
	Mode                string
	Client              kubernetes.Interface
	RequireInventoryGet bool
	CacheTTL            time.Duration
	cache               *authCache
}

// AuthDisabled reports whether auth middleware should be bypassed.
func (a AuthConfig) AuthDisabled() bool {
	switch a.Mode {
	case AuthModeDisabled, "none":
		return true
	default:
		return false
	}
}

// InitCache allocates the in-memory TokenReview/SAR cache when CacheTTL is set.
func (a *AuthConfig) InitCache() {
	if a == nil || a.cache != nil || a.CacheTTL <= 0 {
		return
	}

	a.cache = newAuthCache(a.CacheTTL)
}

func requestAuthScope(r *http.Request) (namespace, name, verb, resource string) {
	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1alpha1/status/targets"):
		return strings.TrimSpace(r.URL.Query().Get("namespace")), "", "list", "kollecttargets"
	case strings.HasPrefix(path, "/v1alpha1/status/inventories"):
		return strings.TrimSpace(r.URL.Query().Get("namespace")), "", "list", "kollectinventories"
	default:
		namespace, name, verb = inventoryAuthScope(r)

		return namespace, name, verb, "kollectinventories"
	}
}

func inventoryAuthScope(r *http.Request) (namespace, name, verb string) {
	namespace = strings.TrimSpace(r.URL.Query().Get("namespace"))
	name = strings.TrimSpace(r.URL.Query().Get("inventory"))

	if ns := r.PathValue("namespace"); ns != "" {
		namespace = strings.TrimSpace(ns)
	}
	if n := r.PathValue("name"); n != "" {
		name = strings.TrimSpace(n)
	}

	if name != "" {
		verb = "get"
	} else {
		verb = "list"
	}

	return namespace, name, verb
}

// Middleware wraps handlers with Kubernetes token validation when enabled.
func (a *AuthConfig) Middleware(next http.Handler) http.Handler {
	if a == nil || a.AuthDisabled() || a.Client == nil {
		return next
	}

	a.InitCache()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)

			return
		}

		namespace, name, verb, resource := requestAuthScope(r)
		hash := tokenHash(token)
		cacheKey := authCacheKey(hash, verb, namespace, name+":"+resource)

		if user, allowed, ok := a.cache.get(cacheKey); ok {
			if a.RequireInventoryGet && !allowed {
				http.Error(w, "forbidden", http.StatusForbidden)

				return
			}

			_ = user
			next.ServeHTTP(w, r)

			return
		}

		user, err := a.authenticate(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		allowed := true
		if a.RequireInventoryGet {
			var authErr error
			allowed, authErr = a.authorizeResource(r.Context(), user, resource, namespace, name, verb)
			if authErr != nil {
				http.Error(w, "authorization check failed", http.StatusInternalServerError)

				return
			}
		}

		a.cache.set(cacheKey, user, allowed)

		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)

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

func (a AuthConfig) authenticate(ctx context.Context, token string) (authenticationv1.UserInfo, error) {
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

func (a AuthConfig) authorizeResource(
	ctx context.Context,
	user authenticationv1.UserInfo,
	resource, namespace, name, verb string,
) (bool, error) {
	attrs := &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Group:     "kollect.dev",
		Resource:  resource,
		Verb:      verb,
	}
	if name != "" {
		attrs.Name = name
	}

	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:               user.Username,
			Groups:             user.Groups,
			ResourceAttributes: attrs,
		},
	}

	result, err := a.Client.AuthorizationV1().SubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("%s SAR: %w", resource, err)
	}

	return result.Status.Allowed, nil
}
