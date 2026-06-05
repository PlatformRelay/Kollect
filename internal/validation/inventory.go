// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const defaultMaxExportBytesGlobal int64 = 1572864 // 1.5 MiB per ADR-0103

var maxExportBytesGlobal int64 = defaultMaxExportBytesGlobal

// SetMaxExportBytesGlobal configures the operator cap for spec.maxExportBytes validation.
func SetMaxExportBytesGlobal(bytes int64) {
	if bytes > 0 {
		maxExportBytesGlobal = bytes
	}
}

// MaxExportBytesGlobal returns the configured global export size cap.
func MaxExportBytesGlobal() int64 {
	return maxExportBytesGlobal
}

// ValidateInventorySpec checks cross-field constraints on KollectInventory spec.
func ValidateInventorySpec(spec *kollectdevv1alpha1.KollectInventorySpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	base := field.NewPath("spec").Child("sinkRefs")

	for i, ref := range spec.SinkRefs {
		allErrs = append(allErrs, validateSameNamespaceRef(ref, base.Index(i), "sinkRef")...)
	}

	if spec.ExportMinInterval != nil && spec.ExportMinInterval.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("exportMinInterval"),
			spec.ExportMinInterval.Duration.String(),
			"must be non-negative",
		))
	}

	if spec.MaxExportBytes != nil {
		fld := field.NewPath("spec").Child("maxExportBytes")
		if *spec.MaxExportBytes <= 0 {
			allErrs = append(allErrs, field.Invalid(fld, *spec.MaxExportBytes, "must be positive when set"))
		} else if *spec.MaxExportBytes > maxExportBytesGlobal {
			allErrs = append(allErrs, field.Invalid(fld, *spec.MaxExportBytes,
				fmt.Sprintf("must not exceed global cap %d bytes", maxExportBytesGlobal)))
		}
	}

	return allErrs
}

// ExportMinIntervalFor returns the effective export debounce for an inventory.
func ExportMinIntervalFor(spec *kollectdevv1alpha1.KollectInventorySpec, fallback time.Duration) time.Duration {
	if spec != nil && spec.ExportMinInterval != nil {
		d := spec.ExportMinInterval.Duration
		if d > 0 {
			return d
		}
	}

	if fallback > 0 {
		return fallback
	}

	return 30 * time.Second
}

// InventoryInvalid formats a validation failure for admission.
func InventoryInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectInventory %q is invalid: %s", name, formatErrors(errs))
}
