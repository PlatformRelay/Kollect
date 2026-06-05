// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateClusterInventorySpec_requiresNamespaceSelector(t *testing.T) {
	t.Parallel()

	errs := ValidateClusterInventorySpec(&kollectdevv1alpha1.KollectClusterInventorySpec{})
	if len(errs) == 0 {
		t.Fatal("expected namespaceSelector required")
	}
}

func TestValidateClusterInventorySpec_rejectsNamespaceNameRefs(t *testing.T) {
	t.Parallel()

	errs := ValidateClusterInventorySpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		TargetRefs: []string{"ns/target"},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"team": "a"},
		},
		SinkRefs: kollectdevv1alpha1.NewSinkRefList("kollect-system/sink"),
	})
	if len(errs) != 2 {
		t.Fatalf("expected ref format errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateClusterInventorySpec_validMinimal(t *testing.T) {
	t.Parallel()

	errs := ValidateClusterInventorySpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"kollect.dev/tenant": "platform"},
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateClusterInventorySpec_rejectsInvalidDedupe(t *testing.T) {
	t.Parallel()

	errs := ValidateClusterInventorySpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"kollect.dev/tenant": "platform"},
		},
		Dedupe: "collapseAll",
	})
	if len(errs) != 1 {
		t.Fatalf("expected 1 dedupe error, got %d: %v", len(errs), errs)
	}
}

func TestValidateClusterInventorySpec_rejectsEmptyTargetRef(t *testing.T) {
	t.Parallel()

	errs := ValidateClusterInventorySpec(&kollectdevv1alpha1.KollectClusterInventorySpec{
		TargetRefs: []string{"  "},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"team": "a"},
		},
	})
	if len(errs) != 1 {
		t.Fatalf("expected empty targetRef error, got %v", errs)
	}
}
