// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateNameOnlyRef(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("sinkRef")
	if errs := validateNameOnlyRef("", path, "sinkRef"); len(errs) != 1 {
		t.Fatalf("empty ref errs = %v", errs)
	}

	if errs := validateNameOnlyRef("team/git", path, "sinkRef"); len(errs) != 1 {
		t.Fatalf("namespaced ref errs = %v", errs)
	}

	if errs := validateNameOnlyRef("demo-git", path, "sinkRef"); len(errs) != 0 {
		t.Fatalf("name-only ref errs = %v", errs)
	}
}

func TestValidateSameNamespaceRef(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("profileRef")
	if errs := validateSameNamespaceRef("", path, "profileRef"); len(errs) != 1 {
		t.Fatalf("empty ref errs = %v", errs)
	}

	if errs := validateSameNamespaceRef("other/profile", path, "profileRef"); len(errs) != 1 {
		t.Fatalf("cross-namespace ref errs = %v", errs)
	}

	if errs := validateSameNamespaceRef("profile-a", path, "profileRef"); len(errs) != 0 {
		t.Fatalf("same-namespace ref errs = %v", errs)
	}
}
