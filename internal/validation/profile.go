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

	// AllowFullResourceExportAnnotation opts a Profile into full-object export for
	// sensitive kinds when export.mode is Resource (ADR-0306 §Security).
	//nolint:gosec // G101: annotation key name, not a credential
	AllowFullResourceExportAnnotation = kollectdevv1alpha1.AllowFullResourceExportAnnotation
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

	warnings = append(warnings, exportWarnings(profile.Spec.Export)...)

	return warnings
}

// exportWarnings flags export.prune features that are accepted but not yet enforced (Phase 1).
func exportWarnings(export *kollectdevv1alpha1.ExportSpec) []string {
	if export == nil || export.Prune == nil {
		return nil
	}

	var warnings []string

	for _, expr := range export.Prune.CEL {
		warnings = append(warnings, fmt.Sprintf(
			"export.prune.cel %q is reserved for Phase 2 and is not yet enforced by the collector",
			expr,
		))
	}

	for _, jp := range export.Prune.JSONPaths {
		if collect.HasJSONPathFilter(jp) || strings.Contains(jp, "[*]") {
			warnings = append(warnings, fmt.Sprintf(
				"export.prune.jsonPaths %q: filter/wildcard expressions are not pruned in Phase 1", jp,
			))
		}
	}

	return warnings
}

// ValidateProfile checks spec, paths, and security policy for a KollectProfile.
func ValidateProfile(profile *kollectdevv1alpha1.KollectProfile) field.ErrorList {
	allErrs := ValidateProfileSpec(&profile.Spec)
	allErrs = append(allErrs, validateSecretDataAccess(profile)...)
	allErrs = append(allErrs, validateExport(profile)...)

	return allErrs
}

// validateExport enforces the export contract for full-resource export (ADR-0306).
func validateExport(profile *kollectdevv1alpha1.KollectProfile) field.ErrorList {
	export := profile.Spec.Export
	if export == nil {
		return nil
	}

	var allErrs field.ErrorList

	exportPath := field.NewPath("spec").Child("export")
	resourceMode := export.ResourceExportEnabled()

	// mode: Attributes (explicit export block) still requires curated attributes.
	if !resourceMode && len(profile.Spec.Attributes) == 0 {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("attributes"),
			"spec.attributes is required when export.mode is Attributes",
		))
	}

	// export.as must not collide with an explicit attribute name (ADR-0306).
	asKey := export.AttributeKey()
	if resourceMode {
		for _, attr := range profile.Spec.Attributes {
			if attr.Name == asKey {
				allErrs = append(allErrs, field.Duplicate(exportPath.Child("as"), asKey))

				break
			}
		}
	}

	if export.Prune != nil {
		ptrPath := exportPath.Child("prune", "jsonPointers")
		for i, ptr := range export.Prune.JSONPointers {
			if err := validateRFC6901Pointer(ptr); err != nil {
				allErrs = append(allErrs, field.Invalid(ptrPath.Index(i), ptr, err.Error()))
			}
		}
	}

	allErrs = append(allErrs, validateFullResourceExportSecret(profile, exportPath)...)

	return allErrs
}

// validateFullResourceExportSecret guards Secret full-object export behind opt-in (ADR-0306 §Security).
func validateFullResourceExportSecret(
	profile *kollectdevv1alpha1.KollectProfile,
	exportPath *field.Path,
) field.ErrorList {
	if !profile.Spec.Export.ResourceExportEnabled() {
		return nil
	}

	if !isSecretTargetGVK(profile.Spec.TargetGVK) {
		return nil
	}

	if allowFullResourceExport(profile.Annotations) {
		return nil
	}

	return field.ErrorList{field.Forbidden(
		exportPath.Child("mode"),
		fmt.Sprintf(
			"Resource export of Secret requires annotation %q: \"true\"",
			AllowFullResourceExportAnnotation,
		),
	)}
}

func allowFullResourceExport(annotations map[string]string) bool {
	return annotations != nil && annotations[AllowFullResourceExportAnnotation] == "true"
}

// validateRFC6901Pointer checks JSON Pointer syntax (RFC 6901): empty, or "/"-prefixed
// tokens where "~" is only followed by 0 or 1.
func validateRFC6901Pointer(pointer string) error {
	pointer = strings.TrimSpace(pointer)
	if pointer == "" {
		return fmt.Errorf("empty JSON pointer")
	}

	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("JSON pointer must start with '/'")
	}

	for i := 0; i < len(pointer); i++ {
		if pointer[i] != '~' {
			continue
		}

		if i+1 >= len(pointer) || (pointer[i+1] != '0' && pointer[i+1] != '1') {
			return fmt.Errorf("invalid escape: '~' must be followed by '0' or '1'")
		}
	}

	return nil
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
