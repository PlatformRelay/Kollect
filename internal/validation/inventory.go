// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

const defaultMaxExportBytesGlobal int64 = 1572864 // 1.5 MiB per ADR-0103

// ExportShardWarnRows is the row count at which operators should split inventories (Q4).
const ExportShardWarnRows = 1800

// maxExportBytesGlobal is read on every validation call and may be reconfigured
// at runtime (SetMaxExportBytesGlobal), so guard it with atomic access to stay
// race-free under concurrent admission webhooks and parallel tests.
var maxExportBytesGlobal atomic.Int64

func init() {
	maxExportBytesGlobal.Store(defaultMaxExportBytesGlobal)
}

// SetMaxExportBytesGlobal configures the operator cap for spec.maxExportBytes validation.
func SetMaxExportBytesGlobal(bytes int64) {
	if bytes > 0 {
		maxExportBytesGlobal.Store(bytes)
	}
}

// MaxExportBytesGlobal returns the configured global export size cap.
func MaxExportBytesGlobal() int64 {
	return maxExportBytesGlobal.Load()
}

// ResolveBindingMaxExportBytes returns the effective export size ceiling for one
// sink binding (AR-01 / EC-P0-01). A per-binding override replaces the fallback
// wholesale when positive — it is an override, not a clamp, so a binding may
// diverge above or below the inventory-wide ceiling. Callers pass the
// inventory-wide ceiling (namespaced) or the operator global cap (cluster) as
// the fallback.
func ResolveBindingMaxExportBytes(override *int64, fallback int64) int64 {
	if override != nil && *override > 0 {
		return *override
	}

	return fallback
}

// ValidateInventorySpec checks cross-field constraints on KollectInventory spec.
func ValidateInventorySpec(spec *kollectdevv1alpha1.KollectInventorySpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, validateInventoryFamilySinkRefs(spec)...)

	if spec.ExportMinInterval != nil {
		allErrs = append(allErrs, ValidateOptionalDurationInterval(
			spec.ExportMinInterval, field.NewPath("spec").Child("exportMinInterval"))...)
	}

	if spec.MaxExportBytes != nil {
		fld := field.NewPath("spec").Child("maxExportBytes")
		if *spec.MaxExportBytes <= 0 {
			allErrs = append(allErrs, field.Invalid(fld, *spec.MaxExportBytes, "must be positive when set"))
		} else if capBytes := maxExportBytesGlobal.Load(); *spec.MaxExportBytes > capBytes {
			allErrs = append(allErrs, field.Invalid(fld, *spec.MaxExportBytes,
				fmt.Sprintf("must not exceed global cap %d bytes", capBytes)))
		}
	}

	return allErrs
}

func validateInventoryFamilySinkRefs(spec *kollectdevv1alpha1.KollectInventorySpec) field.ErrorList {
	total := kollectdevv1alpha1.TotalInventorySinkRefCount(spec)
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

// ExportMinIntervalFor returns the effective inventory-level export debounce default.
func ExportMinIntervalFor(spec *kollectdevv1alpha1.KollectInventorySpec, fallback time.Duration) time.Duration {
	return InventoryDefaultInterval(spec, fallback)
}

// InventoryInvalid formats a validation failure for admission.
func InventoryInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectInventory %q is invalid: %s", name, formatErrors(errs))
}

// ScopeLoadErrors wraps a scope lookup failure for admission responses.
func ScopeLoadErrors(err error) field.ErrorList {
	return field.ErrorList{field.InternalError(field.NewPath("spec"), err)}
}

// ScopeViolationErrors wraps a KollectClusterScope policy violation for admission responses.
func ScopeViolationErrors(err error) field.ErrorList {
	return field.ErrorList{field.Forbidden(field.NewPath("spec"), err.Error())}
}
