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

func TestLoadReturnsOldestScopeByName(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	newer := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "z-scope", Namespace: "team-a"},
	}
	older := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "a-scope", Namespace: "team-a"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(newer, older).Build()
	binding, err := Load(context.Background(), cl, "team-a")
	if err != nil {
		t.Fatal(err)
	}

	if !binding.Enforced || binding.Scope == nil || binding.Scope.Name != "a-scope" {
		t.Fatalf("binding = %+v", binding)
	}
}

func TestLoadEmptyNamespace(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	binding, err := Load(context.Background(), cl, "team-a")
	if err != nil {
		t.Fatal(err)
	}

	if binding.Enforced {
		t.Fatal("expected no enforced scope")
	}
}

func TestValidateTargetGVKCaseInsensitive(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
				{Group: "Apps", Version: "V1", Kind: "Deployment"},
			},
		},
	}

	gvk := kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	if err := ValidateTargetGVK(scope, gvk); err != nil {
		t.Fatalf("expected case-insensitive match: %v", err)
	}
}

func TestValidateTargetGVK(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	gvk := kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	if err := ValidateTargetGVK(scope, gvk); err != nil {
		t.Fatalf("expected allowed GVK: %v", err)
	}

	gvk.Kind = "Pod"
	if err := ValidateTargetGVK(scope, gvk); err == nil {
		t.Fatal("expected GVK violation")
	}
}

func TestValidateTargetGVKNilScopeAllows(t *testing.T) {
	t.Parallel()

	gvk := kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Pod"}
	if err := ValidateTargetGVK(nil, gvk); err != nil {
		t.Fatalf("nil scope should allow: %v", err)
	}
}

func TestValidateSinkRefsNilScope(t *testing.T) {
	t.Parallel()

	if err := ValidateSinkRefs(nil, []string{"demo"}); err != nil {
		t.Fatalf("nil scope should allow: %v", err)
	}
}

func TestValidateSinkRefs(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			SinkRefs: []string{"demo-git"},
		},
	}

	if err := ValidateSinkRefs(scope, []string{"demo-git"}); err != nil {
		t.Fatalf("expected allowed sink: %v", err)
	}

	if err := ValidateSinkRefs(scope, []string{"other"}); err == nil {
		t.Fatal("expected sink violation")
	}
}

func TestValidateWorkloadNamespaces(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"team-a"},
			},
		},
	}

	if err := ValidateWorkloadNamespaces(scope, []string{"team-a"}); err != nil {
		t.Fatalf("expected allowed namespace: %v", err)
	}

	if err := ValidateWorkloadNamespaces(scope, []string{"team-b"}); err == nil {
		t.Fatal("expected namespace violation")
	}
}
