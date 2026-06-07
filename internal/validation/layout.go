// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var (
	layoutPathPlaceholders = map[string]struct{}{
		"cluster": {}, "namespace": {}, "name": {}, "targetNamespace": {}, "targetName": {},
		"sourceNamespace": {}, "sourceName": {}, "group": {}, "kind": {}, "uid": {},
		"generation": {}, "extension": {},
	}
	layoutPathPlaceholderPattern = regexp.MustCompile(`\{([a-zA-Z]+)\}`)
)

// ValidateLayoutSpec checks the structural validity of a snapshot layout block (ADR-0419).
// It does not resolve the referenced profile; manifest-content / Resource-export coupling is
// surfaced as a webhook warning (see LayoutWarnings).
func ValidateLayoutSpec(l *kollectdevv1alpha1.LayoutSpec, path *field.Path) field.ErrorList {
	if l == nil {
		return nil
	}

	var allErrs field.ErrorList

	if l.Mode != "" {
		switch l.Mode {
		case kollectdevv1alpha1.LayoutModeDocument, kollectdevv1alpha1.LayoutModePerResource,
			kollectdevv1alpha1.LayoutModeSplit:
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("mode"), l.Mode, []string{
				kollectdevv1alpha1.LayoutModeDocument,
				kollectdevv1alpha1.LayoutModePerResource,
				kollectdevv1alpha1.LayoutModeSplit,
			}))
		}
	}

	if l.Content != "" {
		switch l.Content {
		case kollectdevv1alpha1.LayoutContentItem, kollectdevv1alpha1.LayoutContentAttributes,
			kollectdevv1alpha1.LayoutContentManifest:
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("content"), l.Content, []string{
				kollectdevv1alpha1.LayoutContentItem,
				kollectdevv1alpha1.LayoutContentAttributes,
				kollectdevv1alpha1.LayoutContentManifest,
			}))
		}
	}

	if err := validateLayoutPathTemplate(l.PathTemplate); err != nil {
		allErrs = append(allErrs, field.Invalid(path.Child("pathTemplate"), l.PathTemplate, err.Error()))
	}

	if l.Index != nil {
		if err := ValidatePathTemplate(l.Index.PathTemplate); err != nil {
			allErrs = append(allErrs, field.Invalid(
				path.Child("index").Child("pathTemplate"), l.Index.PathTemplate, err.Error()))
		}
	}

	if l.Filename != nil {
		if l.Filename.GroupInPath != "" {
			switch l.Filename.GroupInPath {
			case kollectdevv1alpha1.LayoutGroupInPathAuto, kollectdevv1alpha1.LayoutGroupInPathAlways,
				kollectdevv1alpha1.LayoutGroupInPathNever:
			default:
				allErrs = append(allErrs, field.NotSupported(
					path.Child("filename").Child("groupInPath"), l.Filename.GroupInPath, []string{
						kollectdevv1alpha1.LayoutGroupInPathAuto,
						kollectdevv1alpha1.LayoutGroupInPathAlways,
						kollectdevv1alpha1.LayoutGroupInPathNever,
					}))
			}
		}

		if l.Filename.MaxSegmentLength != nil && *l.Filename.MaxSegmentLength < 1 {
			allErrs = append(allErrs, field.Invalid(
				path.Child("filename").Child("maxSegmentLength"), *l.Filename.MaxSegmentLength,
				"must be >= 1"))
		}
	}

	return allErrs
}

// LayoutWarnings returns non-blocking admission warnings for a layout block (ADR-0419).
func LayoutWarnings(l *kollectdevv1alpha1.LayoutSpec) []string {
	if l == nil {
		return nil
	}

	var warns []string
	if strings.EqualFold(l.Content, kollectdevv1alpha1.LayoutContentManifest) {
		warns = append(warns, "layout.content=manifest requires the referenced profile to use export.mode: Resource (ADR-0306); "+
			"export fails per row if the embedded object is absent")
	}

	if l.Mode == kollectdevv1alpha1.LayoutModePerResource || l.Mode == kollectdevv1alpha1.LayoutModeSplit {
		warns = append(warns, "layout.mode="+l.Mode+": git.prune is auto-enabled so stale resource files are removed (ADR-0419)")
	}

	return warns
}

// validateLayoutPathTemplate checks layout.pathTemplate placeholders (ADR-0419).
// Duplicated from internal/sink/layout to keep validation independent of sink.
func validateLayoutPathTemplate(template string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}
	if strings.Contains(template, "..") {
		return fmt.Errorf("layout.pathTemplate must not contain '..'")
	}
	for _, match := range layoutPathPlaceholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		if _, ok := layoutPathPlaceholders[match[1]]; !ok {
			return fmt.Errorf("layout.pathTemplate contains unsupported placeholder {%s}", match[1])
		}
	}
	if !strings.Contains(template, "{sourceName}") && !strings.Contains(template, "{name}") &&
		!strings.Contains(template, "{uid}") {
		return fmt.Errorf("layout.pathTemplate must include a per-resource identifier ({sourceName}, {name}, or {uid})")
	}
	return nil
}
