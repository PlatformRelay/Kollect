// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestLoadClusterReturnsOldestByName(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	newer := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "z-scope"},
	}
	older := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "a-scope"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(newer, older).Build()
	binding, err := LoadCluster(context.Background(), cl)
	if err != nil {
		t.Fatal(err)
	}

	if !binding.Enforced || binding.Scope == nil || binding.Scope.Name != "a-scope" {
		t.Fatalf("binding = %+v", binding)
	}
}

func TestLoadClusterEmpty(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	binding, err := LoadCluster(context.Background(), cl)
	if err != nil {
		t.Fatal(err)
	}

	if binding.Enforced {
		t.Fatal("expected no enforced cluster scope")
	}
}
