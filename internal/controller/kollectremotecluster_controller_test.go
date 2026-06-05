// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectRemoteClusterReconciler_AwaitingFirstReport(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "spoke-a",
			Namespace:  "kollect-system",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{
			ClusterName: "spoke-a",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(rc).
		WithObjects(rc).
		Build()

	r := &KollectRemoteClusterReconciler{Client: c, Scheme: scheme}
	res, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res.RequeueAfter != remoteClusterRequeueInterval {
		t.Fatalf("requeue = %v", res.RequeueAfter)
	}

	var got kollectdevv1alpha1.KollectRemoteCluster
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name}, &got); err != nil {
		t.Fatal(err)
	}

	if got.Status.ObservedGeneration != 1 {
		t.Fatalf("observedGeneration = %d", got.Status.ObservedGeneration)
	}

	cond := findRemoteClusterCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionConnected)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "AwaitingFirstReport" {
		t.Fatalf("Connected = %+v", cond)
	}
}

func findRemoteClusterCondition(conds []metav1.Condition, typ string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == typ {
			return &conds[i]
		}
	}

	return nil
}
