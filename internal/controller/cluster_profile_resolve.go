// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// resolveClusterTargetProfile resolves a cluster target's profileRef to a namespaced
// KollectProfile by explicit namespace + name. No cluster-scoped kind or platform-namespace
// fallback is attempted (ADR-0208).
func resolveClusterTargetProfile(
	ctx context.Context,
	c client.Client,
	profileRef kollectdevv1alpha1.NamespacedObjectReference,
) (*kollectdevv1alpha1.KollectProfile, error) {
	var profile kollectdevv1alpha1.KollectProfile
	key := client.ObjectKey{Name: profileRef.Name, Namespace: profileRef.Namespace}
	if err := c.Get(ctx, key, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(
				"KollectProfile %q not found in namespace %q: %w",
				profileRef.Name, profileRef.Namespace, err,
			)
		}

		return nil, err
	}

	return &profile, nil
}
