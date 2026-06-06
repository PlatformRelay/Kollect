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
	//nolint:gosec // G101: annotation key name, not a credential
	AllowSecretExtractionAnnotation = "kollect.dev/allow-secret-extraction"
)

// ProfileWarnings returns admission warnings for paths that are valid but discouraged (Phase 1).
func ProfileWarnings(profile *kollectdevv1alpha1.KollectProfile) []string {
	var warnings []string

	for _, attr := range profile.Spec.Attributes {
		if collect.HasJSONPathFilter(attr.Path) {
			warnings = append(warnings,
				fmt.Sprintf(
					"attribute %q: JSONPath filter expressions are not supported in Phase 1; "+
						"path %q will be rejected in a future release",
					attr.Name, attr.Path,
				),
			)
		}
	}

	return warnings
}

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

	attrNames := make(map[string]struct{}, len(spec.Attributes))
	for _, attr := range spec.Attributes {
		if attr.Name != "" {
			attrNames[attr.Name] = struct{}{}
		}
	}

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

	allErrs = append(allErrs, validateProfileMetrics(fldPath, spec.Metrics, attrNames)...)

	return allErrs
}

const maxProfileMetricLabels = 5

func validateProfileMetrics(
	fldPath *field.Path,
	metrics []kollectdevv1alpha1.MetricSpec,
	attrNames map[string]struct{},
) field.ErrorList {
	if len(metrics) == 0 {
		return nil
	}

	var allErrs field.ErrorList

	metricPath := fldPath.Child("metrics")
	names := make(map[string]struct{}, len(metrics))

	for i, metric := range metrics {
		idxPath := metricPath.Index(i)

		if metric.Name == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "name is required"))
		} else if _, dup := names[metric.Name]; dup {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), metric.Name))
		} else {
			names[metric.Name] = struct{}{}
		}

		if metric.Path == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("path"), "path is required"))

			continue
		}

		if _, ok := attrNames[metric.Path]; !ok {
			allErrs = append(allErrs, field.Invalid(
				idxPath.Child("path"),
				metric.Path,
				"path must reference an attribute name from spec.attributes",
			))
		}

		if len(metric.Labels) > maxProfileMetricLabels {
			allErrs = append(allErrs, field.TooMany(
				idxPath.Child("labels"),
				len(metric.Labels),
				maxProfileMetricLabels,
			))
		}

		for j, label := range metric.Labels {
			if label == "" {
				allErrs = append(allErrs, field.Required(idxPath.Child("labels").Index(j), "label key is required"))

				continue
			}

			if _, ok := attrNames[label]; !ok {
				allErrs = append(allErrs, field.Invalid(
					idxPath.Child("labels").Index(j),
					label,
					"label must reference an attribute name from spec.attributes",
				))
			}
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
		if !pathRequiresSecretExtractionOptIn(attr.Path) {
			continue
		}

		allErrs = append(allErrs, field.Forbidden(
			attrPath.Index(i).Child("path"),
			fmt.Sprintf(
				"Secret.data paths require annotation %q: \"true\"",
				AllowSecretExtractionAnnotation,
			),
		))
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

func pathRequiresSecretExtractionOptIn(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	if strings.HasPrefix(path, collect.HelmReleasePathPrefix) {
		return collect.HelmReleasePathRequiresSecretOptIn(path)
	}

	return pathTargetsSecretData(path)
}

func pathTargetsSecretData(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	if strings.HasPrefix(path, collect.HelmReleasePathPrefix) {
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

// ScopeInvalid formats a validation failure for admission.
func ScopeInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectScope %q is invalid: %s", name, formatErrors(errs))
}

// ClusterScopeInvalid formats a validation failure for admission.
func ClusterScopeInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterScope %q is invalid: %s", name, formatErrors(errs))
}

func formatErrors(errs field.ErrorList) string {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}

	return strings.Join(msgs, "; ")
}
