// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateTargetSpec checks cross-field constraints on KollectTarget spec.
func ValidateTargetSpec(spec *kollectdevv1alpha1.KollectTargetSpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	errs := validateProfileRef(spec.ProfileRef, field.NewPath("spec").Child("profileRef"))

	return errs
}

func validateProfileRef(ref string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if strings.TrimSpace(ref) == "" {
		errs = append(errs, field.Required(fldPath, "profileRef is required"))
		return errs
	}

	if strings.Contains(ref, "/") {
		errs = append(errs, field.Invalid(fldPath, ref,
			"profileRef must be a profile name in the same namespace as the Target, not namespace/name"))
	}

	return errs
}

// TargetInvalid formats a validation failure for admission.
func TargetInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectTarget %q is invalid: %s", name, formatErrors(errs))
}
