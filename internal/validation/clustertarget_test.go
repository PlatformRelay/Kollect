// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestValidateClusterTargetSpec_namespaceSelectorRequired(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectClusterTargetSpec{
		ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
			Name:      "platform-deployments",
			Namespace: "kollect-system",
		},
	}
	errs := ValidateClusterTargetSpec(&spec)
	if len(errs) == 0 {
		t.Fatal("expected error for missing namespaceSelector")
	}

	spec.NamespaceSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"team": "platform"},
	}
	errs = ValidateClusterTargetSpec(&spec)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateClusterTargetSpec_profileNamespaceRequired(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectClusterTargetSpec{
		ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "platform-deployments"},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"team": "platform"},
		},
	}
	errs := ValidateClusterTargetSpec(&spec)
	if len(errs) == 0 {
		t.Fatal("expected error for missing profileRef.namespace")
	}
}

func TestValidateClusterTargetSpecNil(t *testing.T) {
	t.Parallel()

	if errs := ValidateClusterTargetSpec(nil); len(errs) != 0 {
		t.Fatalf("nil spec errs = %v", errs)
	}
}

// TestNamespaceSelectorEmpty exercises namespaceSelectorEmpty directly, including the
// nil branch that no production caller reaches (they short-circuit on nil first) and
// the MatchExpressions-only path that keeps a selector non-empty (COV-90-S05).
func TestNamespaceSelectorEmpty(t *testing.T) {
	t.Parallel()

	if !namespaceSelectorEmpty(nil) {
		t.Fatal("nil selector must be treated as empty")
	}
	if !namespaceSelectorEmpty(&metav1.LabelSelector{}) {
		t.Fatal("zero-value selector must be treated as empty")
	}
	if namespaceSelectorEmpty(&metav1.LabelSelector{
		MatchLabels: map[string]string{"team": "platform"},
	}) {
		t.Fatal("selector with matchLabels must not be empty")
	}
	if namespaceSelectorEmpty(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "team", Operator: metav1.LabelSelectorOpExists},
		},
	}) {
		t.Fatal("selector with matchExpressions must not be empty")
	}
}

// TestValidateClusterTargetSpec_matchExpressionsSatisfiesSelector confirms a
// matchExpressions-only selector satisfies the required-selector rule end-to-end.
func TestValidateClusterTargetSpec_matchExpressionsSatisfiesSelector(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectClusterTargetSpec{
		ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
			Name:      "platform-deployments",
			Namespace: "kollect-system",
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "team", Operator: metav1.LabelSelectorOpExists},
			},
		},
	}
	if errs := ValidateClusterTargetSpec(&spec); len(errs) != 0 {
		t.Fatalf("matchExpressions selector should satisfy requirement: %v", errs)
	}
}
