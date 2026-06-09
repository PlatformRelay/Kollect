// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateCollectionFilterSpec_RejectsInvalidRulesAndDuplicates(t *testing.T) {
	t.Parallel()

	errs := ValidateCollectionFilterSpec(&kollectdevv1alpha1.CollectionFilterSpec{
		IncludedNamespaces: []string{"apps", "apps"},
		ResourceRules: []kollectdevv1alpha1.ResourceRule{
			{
				GVK: kollectdevv1alpha1.GroupVersionKind{
					Group: "apps",
					Kind:  "",
				},
				MatchPolicy: "metadata.labels[",
			},
		},
	}, field.NewPath("spec"))
	if len(errs) < 2 {
		t.Fatalf("expected multiple validation errors, got %v", errs)
	}
}

func TestValidateScopeCeilingSpec_ValidatesNamespacesAndInterval(t *testing.T) {
	t.Parallel()

	errs := ValidateScopeCeilingSpec(&kollectdevv1alpha1.ScopeCeilingSpec{
		AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
			{Group: "apps", Version: "", Kind: "Deployment"},
		},
		AllowedNamespaces: []string{"team-a", "team-a"},
		DeniedNamespaces:  []string{""},
		MinExportInterval: &metav1.Duration{Duration: 0},
	}, field.NewPath("spec"))
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 validation errors, got %v", errs)
	}
}
