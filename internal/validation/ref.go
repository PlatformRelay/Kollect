// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateNameOnlyRef(ref string, fldPath *field.Path, kind string) field.ErrorList {
	var errs field.ErrorList

	if strings.TrimSpace(ref) == "" {
		errs = append(errs, field.Required(fldPath, kind+" must be a non-empty name"))

		return errs
	}

	if strings.Contains(ref, "/") {
		errs = append(errs, field.Invalid(fldPath, ref,
			kind+" must be a name only, not namespace/name"))
	}

	return errs
}

func validateSameNamespaceRef(ref string, fldPath *field.Path, kind string) field.ErrorList {
	var errs field.ErrorList

	if strings.TrimSpace(ref) == "" {
		errs = append(errs, field.Required(fldPath, kind+" ref is required"))

		return errs
	}

	if strings.Contains(ref, "/") {
		errs = append(errs, field.Invalid(fldPath, ref,
			kind+" ref must be a name in the same namespace, not namespace/name"))
	}

	return errs
}
