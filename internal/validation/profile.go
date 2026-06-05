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

const (
	// AllowSecretExtractionAnnotation opts a Profile into Secret.data extraction paths.
	AllowSecretExtractionAnnotation = "kollect.dev/allow-secret-extraction"
)

// ValidateProfile checks spec, paths, and security policy for a KollectProfile.
func ValidateProfile(profile *kollectdevv1alpha1.KollectProfile) field.ErrorList {
	allErrs := ValidateProfileSpec(&profile.Spec)
	allErrs = append(allErrs, validateSecretDataAccess(profile)...)

	return allErrs
}

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

func validateSecretDataAccess(profile *kollectdevv1alpha1.KollectProfile) field.ErrorList {
	if !isSecretTargetGVK(profile.Spec.TargetGVK) {
		return nil
	}

	if allowSecretExtraction(profile.Annotations) {
		return nil
	}

	attrPath := field.NewPath("spec").Child("attributes")
	var allErrs field.ErrorList

	for i, attr := range profile.Spec.Attributes {
		if pathTargetsSecretData(attr.Path) {
			allErrs = append(allErrs, field.Forbidden(
				attrPath.Index(i).Child("path"),
				fmt.Sprintf(
					"Secret.data paths require annotation %q: \"true\"",
					AllowSecretExtractionAnnotation,
				),
			))
		}
	}

	return allErrs
}

func isSecretTargetGVK(gvk kollectdevv1alpha1.GroupVersionKind) bool {
	if gvk.Kind != "Secret" {
		return false
	}

	switch gvk.Group {
	case "", "core":
		return true
	default:
		return false
	}
}

func allowSecretExtraction(annotations map[string]string) bool {
	return annotations != nil && annotations[AllowSecretExtractionAnnotation] == "true"
}

func pathTargetsSecretData(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	const celPrefix = "cel:"
	if strings.HasPrefix(path, celPrefix) {
		expr := strings.TrimPrefix(path, celPrefix)
		for _, segment := range splitCELPath(expr) {
			if strings.EqualFold(segment, "data") {
				return true
			}
		}

		return false
	}

	rest := path
	switch {
	case strings.HasPrefix(rest, "$."):
		rest = rest[2:]
	case strings.HasPrefix(rest, "$"):
		rest = strings.TrimPrefix(rest, "$")
	}

	rest = strings.TrimPrefix(rest, ".")
	for _, segment := range strings.Split(rest, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		if idx := strings.IndexAny(segment, "[("); idx >= 0 {
			segment = segment[:idx]
		}

		if strings.EqualFold(segment, "data") {
			return true
		}
	}

	return false
}

func splitCELPath(expr string) []string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}

	expr = strings.TrimPrefix(expr, "object")
	expr = strings.TrimPrefix(expr, ".")
	if expr == "" {
		return nil
	}

	parts := strings.FieldsFunc(expr, func(r rune) bool {
		switch r {
		case '.', '[', ']', '(', ')', ' ', '\t':
			return true
		default:
			return false
		}
	})

	return parts
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
