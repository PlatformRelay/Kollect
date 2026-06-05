// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// FindRemoteClusterByClusterName returns the hub registration CR for spec.clusterName.
func FindRemoteClusterByClusterName(
	ctx context.Context,
	c client.Reader,
	platformNamespace,
	clusterName string,
) (*kollectdevv1alpha1.KollectRemoteCluster, error) {
	if c == nil {
		return nil, fmt.Errorf("remote cluster lookup: client is nil")
	}

	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return nil, fmt.Errorf("remote cluster lookup: cluster name is required")
	}

	var list kollectdevv1alpha1.KollectRemoteClusterList
	listOpts := []client.ListOption{}
	if ns := strings.TrimSpace(platformNamespace); ns != "" {
		listOpts = append(listOpts, client.InNamespace(ns))
	}

	if err := c.List(ctx, &list, listOpts...); err != nil {
		return nil, fmt.Errorf("list KollectRemoteCluster: %w", err)
	}

	for i := range list.Items {
		rc := &list.Items[i]
		if strings.TrimSpace(rc.Spec.ClusterName) == clusterName {
			return rc, nil
		}
	}

	return nil, fmt.Errorf("cluster %q is not registered", clusterName)
}

// ValidateTokenClusterBinding ensures the authenticated principal may push for clusterName.
// When annotation kollect.dev/spokePrincipal is set on the registration CR, it must match username.
func ValidateTokenClusterBinding(
	ctx context.Context,
	c client.Reader,
	platformNamespace,
	clusterName,
	username string,
) (*kollectdevv1alpha1.KollectRemoteCluster, error) {
	rc, err := FindRemoteClusterByClusterName(ctx, c, platformNamespace, clusterName)
	if err != nil {
		return nil, err
	}

	if principal := strings.TrimSpace(rc.Annotations[kollectdevv1alpha1.AnnotationSpokePrincipal]); principal != "" {
		if strings.TrimSpace(username) != principal {
			return nil, fmt.Errorf("token principal %q is not bound to cluster %q", username, clusterName)
		}
	}

	return rc, nil
}
