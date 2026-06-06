// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const (
	// DefaultExportMinInterval is the CRD default debounce when no interval is configured.
	DefaultExportMinInterval = 30 * time.Second
	// MaxExportInterval caps duration fields until cron scheduling ships (ADR-0413).
	MaxExportInterval = 24 * time.Hour
	// ZeroIntervalWatchdog requeues inventories whose refs use material-change-only cadence.
	ZeroIntervalWatchdog = 30 * time.Second
	// MaxInventorySinkRefs caps sinkExports cardinality in etcd.
	MaxInventorySinkRefs = 20
)

// ValidateDurationInterval checks a non-negative export interval within the global cap.
func ValidateDurationInterval(d time.Duration, path *field.Path) field.ErrorList {
	if d < 0 {
		return field.ErrorList{field.Invalid(path, d.String(), "must be non-negative")}
	}
	if d > MaxExportInterval {
		return field.ErrorList{field.Invalid(path, d.String(),
			fmt.Sprintf("must not exceed %s without a schedule", MaxExportInterval))}
	}
	return nil
}

// ValidateOptionalDurationInterval validates a pointer duration field when set.
func ValidateOptionalDurationInterval(d *metav1.Duration, path *field.Path) field.ErrorList {
	if d == nil {
		return nil
	}
	return ValidateDurationInterval(d.Duration, path)
}

// ValidateInventorySinkRefs checks structured sinkRefs on an inventory spec.
func ValidateInventorySinkRefs(refs kollectdevv1alpha1.InventorySinkRefList, basePath *field.Path) field.ErrorList {
	if basePath == nil {
		basePath = field.NewPath("spec").Child("sinkRefs")
	}

	var allErrs field.ErrorList
	if len(refs) > MaxInventorySinkRefs {
		allErrs = append(allErrs, field.Invalid(basePath, len(refs),
			fmt.Sprintf("must contain at most %d entries", MaxInventorySinkRefs)))
	}

	seen := make(map[string]struct{}, len(refs))
	for i, ref := range refs {
		refPath := basePath.Index(i)
		allErrs = append(allErrs, validateSameNamespaceRef(ref.Name, refPath.Child("name"), "sinkRef")...)
		if ref.Name != "" {
			if _, dup := seen[ref.Name]; dup {
				allErrs = append(allErrs, field.Duplicate(refPath.Child("name"), ref.Name))
			}
			seen[ref.Name] = struct{}{}
		}
		allErrs = append(allErrs, ValidateOptionalDurationInterval(
			ref.ExportMinInterval, refPath.Child("exportMinInterval"))...)
	}

	return allErrs
}

// InventoryDefaultInterval returns the inventory-level default debounce duration.
func InventoryDefaultInterval(spec *kollectdevv1alpha1.KollectInventorySpec, fallback time.Duration) time.Duration {
	if spec != nil && spec.ExportMinInterval != nil {
		return spec.ExportMinInterval.Duration
	}
	if fallback > 0 {
		return fallback
	}
	return DefaultExportMinInterval
}

// ClusterInventoryDefaultInterval returns the cluster inventory default debounce duration.
func ClusterInventoryDefaultInterval(
	spec *kollectdevv1alpha1.KollectClusterInventorySpec,
	fallback time.Duration,
) time.Duration {
	if spec != nil && spec.ExportMinInterval != nil {
		return spec.ExportMinInterval.Duration
	}
	if fallback > 0 {
		return fallback
	}
	return DefaultExportMinInterval
}

// ScopeMinExportInterval returns the scope floor when configured.
func ScopeMinExportInterval(scope *kollectdevv1alpha1.KollectScope) time.Duration {
	if scope == nil || scope.Spec.MinExportInterval == nil {
		return 0
	}
	return scope.Spec.MinExportInterval.Duration
}

// ScopeCeilingMinExportInterval returns the floor from a ScopeCeilingSpec.
func ScopeCeilingMinExportInterval(ceiling *kollectdevv1alpha1.ScopeCeilingSpec) time.Duration {
	if ceiling == nil || ceiling.MinExportInterval == nil {
		return 0
	}
	return ceiling.MinExportInterval.Duration
}

// ResolveSinkExportInterval computes the effective debounce for one sink ref (ADR-0413 precedence).
func ResolveSinkExportInterval(
	ref kollectdevv1alpha1.InventorySinkRef,
	sinkExportMinInterval *metav1.Duration,
	inventoryDefault time.Duration,
	scopeFloor time.Duration,
) time.Duration {
	var chosen time.Duration
	switch {
	case ref.ExportMinInterval != nil:
		chosen = ref.ExportMinInterval.Duration
	case sinkExportMinInterval != nil:
		chosen = sinkExportMinInterval.Duration
	default:
		chosen = inventoryDefault
	}

	if scopeFloor > 0 && (chosen == 0 || chosen < scopeFloor) {
		return scopeFloor
	}
	return chosen
}

// ValidateIntervalsAgainstScopeFloor rejects explicit intervals below the scope minimum.
func ValidateIntervalsAgainstScopeFloor(
	inventoryDefault *metav1.Duration,
	refLists []kollectdevv1alpha1.InventorySinkRefList,
	floor time.Duration,
) field.ErrorList {
	if floor <= 0 {
		return nil
	}

	var allErrs field.ErrorList
	defaultPath := field.NewPath("spec").Child("exportMinInterval")
	if inventoryDefault != nil && intervalBelowFloor(inventoryDefault.Duration, floor) {
		allErrs = append(allErrs, field.Invalid(defaultPath, inventoryDefault.Duration.String(),
			fmt.Sprintf("must be at least scope minExportInterval %s", floor)))
	}

	for _, refs := range refLists {
		for i, ref := range refs {
			if ref.ExportMinInterval == nil {
				continue
			}
			if intervalBelowFloor(ref.ExportMinInterval.Duration, floor) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("sinkRefs").Index(i).Child("exportMinInterval"),
					ref.ExportMinInterval.Duration.String(),
					fmt.Sprintf("must be at least scope minExportInterval %s", floor)))
			}
		}
	}

	return allErrs
}

// ValidateSinkIntervalAgainstScopeFloor rejects sink defaults below the scope minimum.
func ValidateSinkIntervalAgainstScopeFloor(interval *metav1.Duration, floor time.Duration) field.ErrorList {
	if floor <= 0 || interval == nil {
		return nil
	}
	if intervalBelowFloor(interval.Duration, floor) {
		return field.ErrorList{field.Invalid(field.NewPath("spec").Child("exportMinInterval"),
			interval.Duration.String(),
			fmt.Sprintf("must be at least scope minExportInterval %s", floor))}
	}
	return nil
}

func intervalBelowFloor(interval, floor time.Duration) bool {
	return interval < floor
}

// RequeueAfterForZeroInterval returns the watchdog delay for material-change-only refs.
func RequeueAfterForZeroInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return ZeroIntervalWatchdog
}
