// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateClusterProfileReusesProfileRules(t *testing.T) {
	t.Parallel()

	profile := &kollectdevv1alpha1.KollectClusterProfile{
		Spec: kollectdevv1alpha1.KollectClusterProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{
				Version: "v1",
				Kind:    "Deployment",
			},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "image", Path: "$.spec.template.spec.containers[0].image"},
			},
		},
	}

	if errs := ValidateClusterProfile(profile); len(errs) > 0 {
		t.Fatalf("expected valid cluster profile, got %v", errs)
	}

	profile.Spec.TargetGVK.Kind = ""
	if errs := ValidateClusterProfile(profile); len(errs) == 0 {
		t.Fatal("expected kind required error")
	}
}
