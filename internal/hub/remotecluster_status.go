// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// MarkRemoteClusterConnected sets Connected=True on KollectRemoteCluster resources matching clusterName.
func MarkRemoteClusterConnected(ctx context.Context, c client.Client, clusterName string) error {
	if c == nil {
		return fmt.Errorf("mark remote cluster connected: client is nil")
	}

	clusterName = trimCluster(clusterName)
	if clusterName == "" {
		return fmt.Errorf("mark remote cluster connected: cluster name is required")
	}

	var list kollectdevv1alpha1.KollectRemoteClusterList
	if err := c.List(ctx, &list); err != nil {
		return fmt.Errorf("list KollectRemoteCluster: %w", err)
	}

	updated := false
	for i := range list.Items {
		rc := &list.Items[i]
		if trimCluster(rc.Spec.ClusterName) != clusterName {
			continue
		}

		connected := apimeta.FindStatusCondition(rc.Status.Conditions, kollectdevv1alpha1.ConditionConnected)
		if connected != nil &&
			connected.Status == metav1.ConditionTrue &&
			connected.Reason == "ReportReceived" {
			continue
		}

		apimeta.SetStatusCondition(&rc.Status.Conditions, metav1.Condition{
			Type:               kollectdevv1alpha1.ConditionConnected,
			Status:             metav1.ConditionTrue,
			Reason:             "ReportReceived",
			Message:            fmt.Sprintf("spoke report received for cluster %q", clusterName),
			ObservedGeneration: rc.Generation,
			LastTransitionTime: metav1.Now(),
		})

		if err := c.Status().Update(ctx, rc); err != nil {
			return fmt.Errorf("update KollectRemoteCluster %s/%s status: %w",
				rc.Namespace, rc.Name, err)
		}

		updated = true
	}

	if !updated {
		return nil
	}

	return nil
}

func trimCluster(s string) string {
	return strings.TrimSpace(s)
}
