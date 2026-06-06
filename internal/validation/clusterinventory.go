// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateClusterInventorySpec checks KollectClusterInventory spec fields.
func ValidateClusterInventorySpec(spec *kollectdevv1alpha1.KollectClusterInventorySpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList

	if strings.TrimSpace(spec.ProfileRef) != "" {
		allErrs = append(allErrs, validateNameOnlyRef(
			spec.ProfileRef,
			field.NewPath("spec").Child("profileRef"),
			"profileRef",
		)...)
	}

	targetRefsPath := field.NewPath("spec").Child("targetRefs")
	for i, ref := range spec.TargetRefs {
		allErrs = append(allErrs, validateNameOnlyRef(ref, targetRefsPath.Index(i), "targetRef")...)
	}

	nsPath := field.NewPath("spec").Child("namespaceSelector")
	if spec.NamespaceSelector == nil || namespaceSelectorEmpty(spec.NamespaceSelector) {
		allErrs = append(allErrs, field.Required(nsPath,
			"namespaceSelector is required — empty selector would rollup cluster-wide"))
	}

	allErrs = append(allErrs, validateClusterInventoryFamilySinkRefs(spec)...)

	if strings.TrimSpace(spec.SinkNamespace) != "" && strings.Contains(spec.SinkNamespace, "/") {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("sinkNamespace"),
			spec.SinkNamespace,
			"sinkNamespace must be a single namespace name, not namespace/name",
		))
	}

	if spec.ExportMinInterval != nil {
		allErrs = append(allErrs, ValidateOptionalDurationInterval(
			spec.ExportMinInterval, field.NewPath("spec").Child("exportMinInterval"))...)
	}

	dedupePath := field.NewPath("spec").Child("dedupe")
	switch spec.Dedupe {
	case "", kollectdevv1alpha1.ClusterInventoryDedupeKeepAll, kollectdevv1alpha1.ClusterInventoryDedupeByResourceUID:
	default:
		allErrs = append(allErrs, field.NotSupported(dedupePath, spec.Dedupe,
			[]string{kollectdevv1alpha1.ClusterInventoryDedupeKeepAll, kollectdevv1alpha1.ClusterInventoryDedupeByResourceUID}))
	}

	return allErrs
}

func validateClusterInventoryFamilySinkRefs(spec *kollectdevv1alpha1.KollectClusterInventorySpec) field.ErrorList {
	total := kollectdevv1alpha1.TotalClusterInventorySinkRefCount(spec)
	if total > MaxInventorySinkRefs {
		return field.ErrorList{field.Invalid(field.NewPath("spec"), total,
			fmt.Sprintf("combined family sink refs must contain at most %d entries", MaxInventorySinkRefs))}
	}

	allErrs := make(field.ErrorList, 0, 3)
	allErrs = append(allErrs, ValidateInventorySinkRefs(spec.SnapshotSinkRefs,
		field.NewPath("spec").Child("snapshotSinkRefs"))...)
	allErrs = append(allErrs, ValidateInventorySinkRefs(spec.DatabaseSinkRefs,
		field.NewPath("spec").Child("databaseSinkRefs"))...)
	allErrs = append(allErrs, ValidateInventorySinkRefs(spec.EventSinkRefs,
		field.NewPath("spec").Child("eventSinkRefs"))...)

	return allErrs
}

// ClusterExportMinIntervalFor returns the effective cluster inventory export debounce default.
func ClusterExportMinIntervalFor(
	spec *kollectdevv1alpha1.KollectClusterInventorySpec,
	fallback time.Duration,
) time.Duration {
	return ClusterInventoryDefaultInterval(spec, fallback)
}

// ClusterInventoryInvalid formats a validation failure for admission.
func ClusterInventoryInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterInventory %q is invalid: %s", name, formatErrors(errs))
}
