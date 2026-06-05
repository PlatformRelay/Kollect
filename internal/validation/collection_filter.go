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

// ValidateCollectionFilterSpec checks collection intent fields shared by Target kinds.
func ValidateCollectionFilterSpec(
	filter *kollectdevv1alpha1.CollectionFilterSpec,
	basePath *field.Path,
) field.ErrorList {
	if filter == nil {
		return nil
	}

	if basePath == nil {
		basePath = field.NewPath("spec")
	}

	var allErrs field.ErrorList

	allErrs = append(allErrs, validateUniqueNonEmptyStringsField(
		filter.IncludedNamespaces, basePath.Child("includedNamespaces"))...)
	allErrs = append(allErrs, validateUniqueNonEmptyStringsField(
		filter.ExcludedNamespaces, basePath.Child("excludedNamespaces"))...)

	rulesPath := basePath.Child("resourceRules")
	for i, rule := range filter.ResourceRules {
		rulePath := rulesPath.Index(i)
		if rule.GVK.Version == "" || rule.GVK.Kind == "" {
			allErrs = append(allErrs, field.Required(rulePath.Child("gvk"), "version and kind are required"))
		}

		if strings.TrimSpace(rule.MatchPolicy) != "" {
			if err := collect.ValidateMatchPolicyExpression(rule.MatchPolicy); err != nil {
				allErrs = append(allErrs, field.Invalid(rulePath.Child("matchPolicy"), rule.MatchPolicy, err.Error()))
			}
		}
	}

	return allErrs
}

// ValidateScopeCeilingSpec checks scope ceiling fields.
func ValidateScopeCeilingSpec(spec *kollectdevv1alpha1.ScopeCeilingSpec, basePath *field.Path) field.ErrorList {
	if spec == nil {
		return nil
	}

	if basePath == nil {
		basePath = field.NewPath("spec")
	}

	var allErrs field.ErrorList

	for i, gvk := range spec.AllowedGVKs {
		if gvk.Version == "" || gvk.Kind == "" {
			allErrs = append(allErrs, field.Required(basePath.Child("allowedGVKs").Index(i),
				"version and kind are required"))
		}
	}

	allErrs = append(allErrs, validateUniqueNonEmptyStringsField(
		spec.AllowedNamespaces, basePath.Child("allowedNamespaces"))...)
	allErrs = append(allErrs, validateUniqueNonEmptyStringsField(
		spec.DeniedNamespaces, basePath.Child("deniedNamespaces"))...)
	allErrs = append(allErrs, ValidateOptionalDurationInterval(
		spec.MinExportInterval, basePath.Child("minExportInterval"))...)

	return allErrs
}

func validateUniqueNonEmptyStringsField(values []string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	seen := make(map[string]struct{}, len(values))

	for i, value := range values {
		if value == "" {
			allErrs = append(allErrs, field.Invalid(path.Index(i), value, "must not be empty"))
			continue
		}

		if _, ok := seen[value]; ok {
			allErrs = append(allErrs, field.Invalid(path.Index(i), value, fmt.Sprintf("duplicate entry %q", value)))
			continue
		}

		seen[value] = struct{}{}
	}

	return allErrs
}
