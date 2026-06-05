// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateTargetGVK(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
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
			AllowedNamespaces: []string{"team-a"},
		},
	}

	if err := ValidateWorkloadNamespaces(scope, []string{"team-a"}); err != nil {
		t.Fatalf("expected allowed namespace: %v", err)
	}

	if err := ValidateWorkloadNamespaces(scope, []string{"team-b"}); err == nil {
		t.Fatal("expected namespace violation")
	}
}
