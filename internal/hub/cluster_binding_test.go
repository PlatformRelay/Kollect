// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/hub"
)

func TestValidateTokenClusterBinding(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spoke-a",
			Namespace: "platform",
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationSpokePrincipal: "system:serviceaccount:spoke-a:kollect-spoke",
			},
		},
		Spec: kollectdevv1alpha1.KollectRemoteClusterSpec{ClusterName: "spoke-a"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rc).Build()

	if _, err := hub.ValidateTokenClusterBinding(
		context.Background(),
		cl,
		"platform",
		"spoke-a",
		"system:serviceaccount:spoke-a:kollect-spoke",
	); err != nil {
		t.Fatalf("expected bound principal: %v", err)
	}

	if _, err := hub.ValidateTokenClusterBinding(
		context.Background(),
		cl,
		"platform",
		"spoke-a",
		"system:serviceaccount:rogue:kollect-spoke",
	); err == nil {
		t.Fatal("expected principal mismatch error")
	}

	if _, err := hub.ValidateTokenClusterBinding(
		context.Background(),
		cl,
		"platform",
		"rogue",
		"system:serviceaccount:spoke-a:kollect-spoke",
	); err == nil {
		t.Fatal("expected unregistered cluster error")
	}
}
