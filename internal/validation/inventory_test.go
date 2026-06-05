// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestMaxExportBytesGlobalDefault(t *testing.T) {
	t.Parallel()

	if MaxExportBytesGlobal() <= 0 {
		t.Fatalf("global cap = %d", MaxExportBytesGlobal())
	}

	SetMaxExportBytesGlobal(0)
	if MaxExportBytesGlobal() <= 0 {
		t.Fatal("zero override should not change cap")
	}
}

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

func TestValidateInventorySpecSinkRefAndIntervals(t *testing.T) {
	t.Parallel()

	if ValidateInventorySpec(nil) != nil {
		t.Fatal("nil spec should produce no errors")
	}

	errs := ValidateInventorySpec(&kollectdevv1alpha1.KollectInventorySpec{
		SinkRefs: []string{"other/sink"},
	})
	if len(errs) != 1 {
		t.Fatalf("sink ref errs = %v", errs)
	}

	neg := metav1.Duration{Duration: -time.Second}
	errs = ValidateInventorySpec(&kollectdevv1alpha1.KollectInventorySpec{
		ExportMinInterval: &neg,
	})
	if len(errs) != 1 {
		t.Fatalf("negative interval errs = %v", errs)
	}

	zero := int64(0)
	errs = ValidateInventorySpec(&kollectdevv1alpha1.KollectInventorySpec{
		MaxExportBytes: &zero,
	})
	if len(errs) != 1 {
		t.Fatalf("zero maxExportBytes errs = %v", errs)
	}
}

func TestExportMinIntervalFor(t *testing.T) {
	t.Parallel()

	if got := ExportMinIntervalFor(nil, 0); got != 30*time.Second {
		t.Fatalf("default = %v", got)
	}

	custom := metav1.Duration{Duration: 45 * time.Second}
	if got := ExportMinIntervalFor(&kollectdevv1alpha1.KollectInventorySpec{
		ExportMinInterval: &custom,
	}, 0); got != 45*time.Second {
		t.Fatalf("custom = %v", got)
	}
}

func TestInventoryInvalid(t *testing.T) {
	t.Parallel()

	err := InventoryInvalid("demo", field.ErrorList{
		field.Required(field.NewPath("spec").Child("sinkRefs"), "required"),
	})
	if err == nil || !strings.Contains(err.Error(), "demo") {
		t.Fatalf("err = %v", err)
	}
}
