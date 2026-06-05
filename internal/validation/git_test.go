// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateGitSpec_rejectsInvalidBranch(t *testing.T) {
	t.Parallel()

	errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{Branch: "-evil"},
	})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateGitSinkWarnings_forcePush(t *testing.T) {
	t.Parallel()

	warnings := ValidateGitSinkWarnings(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{PushPolicy: kollectdevv1alpha1.GitPushPolicyForcePush},
	})
	if len(warnings) != 1 {
		t.Fatalf("expected warning, got %v", warnings)
	}
}
