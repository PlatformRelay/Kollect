// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func resolveClusterTargetProfile(
	ctx context.Context,
	c client.Client,
	profileRef string,
) (*kollectdevv1alpha1.KollectProfile, error) {
	var clusterProfile kollectdevv1alpha1.KollectClusterProfile
	if err := c.Get(ctx, client.ObjectKey{Name: profileRef}, &clusterProfile); err == nil {
		return clusterProfileAsProfile(&clusterProfile), nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	var profile kollectdevv1alpha1.KollectProfile
	key := client.ObjectKey{Name: profileRef, Namespace: sink.DefaultSecretNamespace}
	if err := c.Get(ctx, key, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(
				"profile %q not found as KollectClusterProfile or KollectProfile in %q",
				profileRef, sink.DefaultSecretNamespace,
			)
		}

		return nil, err
	}

	return &profile, nil
}

func clusterProfileAsProfile(cp *kollectdevv1alpha1.KollectClusterProfile) *kollectdevv1alpha1.KollectProfile {
	return &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: cp.ObjectMeta,
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK:  cp.Spec.TargetGVK,
			Attributes: cp.Spec.Attributes,
		},
	}
}
