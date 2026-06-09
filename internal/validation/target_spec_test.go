// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateTargetSpec_CombinesProfileRefAndCollectionFilterErrors(t *testing.T) {
	t.Parallel()

	errs := ValidateTargetSpec(&kollectdevv1alpha1.KollectTargetSpec{
		ProfileRef: "other-ns/profile-a",
		CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
			ResourceRules: []kollectdevv1alpha1.ResourceRule{
				{
					GVK: kollectdevv1alpha1.GroupVersionKind{
						Group: "apps",
						Kind:  "Deployment",
					},
				},
			},
		},
	})

	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestTargetInvalid_FormatsNameAndFields(t *testing.T) {
	t.Parallel()

	err := TargetInvalid("apps-target", field.ErrorList{
		field.Invalid(field.NewPath("spec").Child("profileRef"), "other-ns/profile", "must be same namespace"),
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `KollectTarget "apps-target" is invalid`) {
		t.Fatalf("unexpected error message: %q", msg)
	}
	if !strings.Contains(msg, "spec.profileRef") {
		t.Fatalf("expected field path in error: %q", msg)
	}
}
