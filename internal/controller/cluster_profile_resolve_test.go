// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func TestResolveClusterTargetProfile_clusterProfile(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}

	cp := &kollectdevv1alpha1.KollectClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments"},
		Spec: kollectdevv1alpha1.KollectClusterProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Deployment"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp).Build()
	got, err := resolveClusterTargetProfile(context.Background(), c, "deployments")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if got.Spec.TargetGVK.Kind != "Deployment" {
		t.Fatalf("kind = %q", got.Spec.TargetGVK.Kind)
	}
}

func TestResolveClusterTargetProfile_platformProfileFallback(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "argo", Namespace: sink.DefaultSecretNamespace},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1alpha1", Kind: "Application"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(profile).Build()
	got, err := resolveClusterTargetProfile(context.Background(), c, "argo")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if got.Name != "argo" {
		t.Fatalf("name = %q", got.Name)
	}
}
