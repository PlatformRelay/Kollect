// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateInventorySpec_maxExportBytesCap(t *testing.T) {
	t.Parallel()

	SetMaxExportBytesGlobal(1000)
	t.Cleanup(func() { SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal) })

	over := int64(2000)
	spec := &kollectdevv1alpha1.KollectInventorySpec{MaxExportBytes: &over}
	errs := ValidateInventorySpec(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for maxExportBytes above global cap")
	}
}
