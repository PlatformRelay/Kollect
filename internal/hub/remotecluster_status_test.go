// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/hub"
)

func TestMarkRemoteClusterConnected(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spoke-a",
			Namespace: "kollect-system",
		},
		Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{
			ClusterName: "spoke-a",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(rc).WithObjects(rc).Build()

	if err := hub.MarkRemoteClusterConnected(context.Background(), c, "spoke-a"); err != nil {
		t.Fatalf("mark connected: %v", err)
	}

	var got kollectdevv1alpha1.KollectRemoteCluster
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(rc), &got); err != nil {
		t.Fatal(err)
	}

	cond := findCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionConnected)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("Connected = %+v", cond)
	}
}

func findCondition(conds []metav1.Condition, typ string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == typ {
			return &conds[i]
		}
	}

	return nil
}
