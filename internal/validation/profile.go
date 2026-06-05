// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// ValidateProfileSpec checks target GVK and attribute paths (CEL compile + JSONPath syntax).
func ValidateProfileSpec(spec *kollectdevv1alpha1.KollectProfileSpec) field.ErrorList {
	var allErrs field.ErrorList

	fldPath := field.NewPath("spec")

	if spec.TargetGVK.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("targetGVK", "version"), "version is required"))
	}

	if spec.TargetGVK.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("targetGVK", "kind"), "kind is required"))
	}

	extractor, err := collect.NewExtractor()
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("init extractor: %w", err)))

		return allErrs
	}

	names := make(map[string]struct{}, len(spec.Attributes))
	attrPath := fldPath.Child("attributes")

	for i, attr := range spec.Attributes {
		idxPath := attrPath.Index(i)

		if attr.Name == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "name is required"))
		} else if _, dup := names[attr.Name]; dup {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), attr.Name))
		} else {
			names[attr.Name] = struct{}{}
		}

		if attr.Path == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("path"), "path is required"))

			continue
		}

		if err := collect.ValidateAttributePath(extractor, attr.Path); err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("path"), attr.Path, err.Error()))
		}
	}

	return allErrs
}

// ProfileInvalid formats a validation failure for admission.
func ProfileInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectProfile %q is invalid: %s", name, formatErrors(errs))
}

func formatErrors(errs field.ErrorList) string {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}

	return strings.Join(msgs, "; ")
}
