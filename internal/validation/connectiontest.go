// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateConnectionTestSpec checks KollectConnectionTest spec fields.
func ValidateConnectionTestSpec(spec *kollectdevv1alpha1.KollectConnectionTestSpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, validateSameNamespaceRef(
		spec.SinkRef,
		field.NewPath("spec").Child("sinkRef"),
		"sinkRef",
	)...)

	if spec.ProfileRef != "" {
		allErrs = append(allErrs, validateSameNamespaceRef(
			spec.ProfileRef,
			field.NewPath("spec").Child("profileRef"),
			"profileRef",
		)...)
	}

	return allErrs
}

// ConnectionTestInvalid formats a validation failure for admission.
func ConnectionTestInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectConnectionTest %q is invalid: %s", name, formatErrors(errs))
}
