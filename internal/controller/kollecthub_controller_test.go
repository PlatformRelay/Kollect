// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectHubReconciler_WiresRemoteClusters(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-a", Namespace: "platform"},
		Spec:       kollectdevv1alpha1.KollectRemoteClusterSpec{ClusterName: "spoke-a"},
	}

	hub := &kollectdevv1alpha1.KollectHub{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Generation: 1},
		Spec: kollectdevv1alpha1.KollectHubSpec{
			Transport: kollectdevv1alpha1.HubTransportSpec{Type: "inprocess"},
			RemoteClusters: []kollectdevv1alpha1.RemoteClusterRef{{
				Name:      "spoke-a",
				Namespace: "platform",
			}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(hub).
		WithObjects(hub, rc).
		Build()

	r := &KollectHubReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: hub.Name},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	wire := r.formatRemoteClusterWire(context.Background(), hub)
	if wire != "platform/spoke-a:spoke-a" {
		t.Fatalf("wire = %q", wire)
	}

	var got kollectdevv1alpha1.KollectHub
	if err := c.Get(context.Background(), types.NamespacedName{Name: hub.Name}, &got); err != nil {
		t.Fatal(err)
	}

	if got.Status.RegisteredRemoteClusters != 1 {
		t.Fatalf("registered = %d", got.Status.RegisteredRemoteClusters)
	}
}
