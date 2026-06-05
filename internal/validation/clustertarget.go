// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateClusterTargetSpec checks KollectClusterTarget spec fields.
func ValidateClusterTargetSpec(spec *kollectdevv1alpha1.KollectClusterTargetSpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	profilePath := field.NewPath("spec").Child("profileRef")

	if strings.TrimSpace(spec.ProfileRef) == "" {
		allErrs = append(allErrs, field.Required(profilePath, "profileRef is required"))
	} else if strings.Contains(spec.ProfileRef, "/") {
		allErrs = append(allErrs, field.Invalid(profilePath, spec.ProfileRef,
			"profileRef must be a profile name in the platform namespace, not namespace/name"))
	}

	nsPath := field.NewPath("spec").Child("namespaceSelector")
	if spec.NamespaceSelector == nil || namespaceSelectorEmpty(spec.NamespaceSelector) {
		allErrs = append(allErrs, field.Required(nsPath,
			"namespaceSelector is required — empty selector would collect cluster-wide"))
	}

	return allErrs
}

func namespaceSelectorEmpty(sel *metav1.LabelSelector) bool {
	if sel == nil {
		return true
	}

	return len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0
}

// ClusterTargetInvalid formats a validation failure for admission.
func ClusterTargetInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterTarget %q is invalid: %s", name, formatErrors(errs))
}
