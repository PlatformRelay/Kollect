// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateTargetSpec checks cross-field constraints on KollectTarget spec.
func ValidateTargetSpec(spec *kollectdevv1alpha1.KollectTargetSpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	return validateSameNamespaceRef(spec.ProfileRef, field.NewPath("spec").Child("profileRef"), "profileRef")
}

// TargetInvalid formats a validation failure for admission.
func TargetInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectTarget %q is invalid: %s", name, formatErrors(errs))
}
