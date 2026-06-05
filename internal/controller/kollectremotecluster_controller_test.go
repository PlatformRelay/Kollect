// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/remotecredentials"
	"github.com/konih/kollect/internal/remotesecret"
)

type noopAPIChecker struct{}

func (noopAPIChecker) Check(_ context.Context, _ []byte) error { return nil }

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

func TestKollectRemoteClusterReconciler_CredentialsVerified(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	yaml, err := remotesecret.GenerateYAML(remotesecret.Options{
		ClusterName: "spoke-a",
		Namespace:   "platform",
		APIServer:   "https://spoke.example:6443",
		Token:       "test-token",
		CAData:      base64.StdEncoding.EncodeToString([]byte("ca")),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Extract base64 payload from generated YAML (test-only shortcut).
	const marker = "  spoke-a: "
	start := strings.Index(yaml, marker)
	if start < 0 {
		t.Fatal("marker not found in generated yaml")
	}
	start += len(marker)
	end := strings.Index(yaml[start:], "\n")
	if end < 0 {
		t.Fatal("newline not found after marker")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(yaml[start : start+end]))
	if err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kollect-remote-secret-spoke-a",
			Namespace: "platform",
		},
		Data: map[string][]byte{"spoke-a": raw},
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "spoke-a",
			Namespace:  "platform",
			Generation: 1,
		},
		Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{
			ClusterName: "spoke-a",
			CredentialsSecretRef: &corev1.LocalObjectReference{
				Name: secret.Name,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(rc).
		WithObjects(rc, secret).
		Build()

	r := &KollectRemoteClusterReconciler{
		Client:     c,
		Scheme:     scheme,
		APIChecker: noopAPIChecker{},
	}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectRemoteCluster
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: rc.Namespace, Name: rc.Name}, &got); err != nil {
		t.Fatal(err)
	}

	cred := findRemoteClusterCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionCredentialsVerified)
	if cred == nil || cred.Status != metav1.ConditionTrue || cred.Reason != "CredentialsVerified" {
		t.Fatalf("CredentialsVerified = %+v", cred)
	}

	// Sanity: package-level verify matches controller path.
	if err := remotecredentials.VerifySecret(context.Background(), secret, "spoke-a", noopAPIChecker{}); err != nil {
		t.Fatalf("VerifySecret: %v", err)
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
