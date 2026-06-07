// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateOptionsMap_rejectsSecretLikeKeys(t *testing.T) {
	errs := ValidateOptionsMap(map[string]string{"password": "x"}, nil)
	if len(errs) == 0 {
		t.Fatal("expected forbidden error for password key")
	}
}

func TestValidateSinkFormatCapability_rejectsParquetOnKafka(t *testing.T) {
	errs := ValidateSinkFormatCapability(
		kollectdevv1alpha1.EventSinkTypeKafka,
		kollectdevv1alpha1.SerializationFormatParquet,
		nil,
	)
	if len(errs) == 0 {
		t.Fatal("expected unsupported format error")
	}
}

func TestValidateSinkConfigWarnings_existingMode(t *testing.T) {
	warns := ValidateSinkConfigWarnings(&kollectdevv1alpha1.KollectSinkSpec{
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
	})
	if len(warns) == 0 {
		t.Fatal("expected warning for existing mode")
	}
}
