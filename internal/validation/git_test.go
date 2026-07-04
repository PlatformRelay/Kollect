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

func TestValidateGitRef_rejectsBranches(t *testing.T) {
	t.Parallel()

	// empty ref (line 21-23)
	if err := ValidateGitRef(""); err == nil {
		t.Fatal("empty ref must fail")
	}

	// .lock suffix (line 33-35)
	if err := ValidateGitRef("branch.lock"); err == nil {
		t.Fatal(".lock suffix must fail")
	}

	// trailing dot (line 33-35)
	if err := ValidateGitRef("branch."); err == nil {
		t.Fatal("trailing dot must fail")
	}

	// @{ contained (line 36-38)
	if err := ValidateGitRef("branch@{foo}"); err == nil {
		t.Fatal("@{ must fail")
	}

	// bare @ (line 36-38)
	if err := ValidateGitRef("@"); err == nil {
		t.Fatal("bare @ must fail")
	}
}

func TestValidateGitSpec_branches(t *testing.T) {
	t.Parallel()

	// nil or non-git → nil (line 49 first clause)
	if errs := validateGitSpec(nil); errs != nil {
		t.Fatalf("nil spec: %v", errs)
	}
	if errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"}); errs != nil {
		t.Fatalf("non-git type: %v", errs)
	}

	// invalid pushPolicy (line 64-70)
	errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{PushPolicy: "squash"},
	})
	if len(errs) != 1 {
		t.Fatalf("bad pushPolicy: expected 1 error, got %v", errs)
	}

	// invalid auth type (line 74-81)
	errs = validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git: &kollectdevv1alpha1.GitSpec{
			Auth: &kollectdevv1alpha1.GitAuthSpec{Type: "basic"},
		},
	})
	if len(errs) != 1 {
		t.Fatalf("bad auth type: expected 1 error, got %v", errs)
	}

	// secretRef with empty name (line 83-85)
	errs = validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git: &kollectdevv1alpha1.GitSpec{
			Auth: &kollectdevv1alpha1.GitAuthSpec{
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: ""},
			},
		},
	})
	if len(errs) != 1 {
		t.Fatalf("empty secretRef name: expected 1 error, got %v", errs)
	}

	// invalid engine (line 88-97)
	errs = validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{Engine: "libgit2"},
	})
	if len(errs) != 1 {
		t.Fatalf("bad engine: expected 1 error, got %v", errs)
	}

	// cloneDepth < 1 (line 100-102)
	depth := int32(0)
	errs = validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{CloneDepth: &depth},
	})
	if len(errs) != 1 {
		t.Fatalf("cloneDepth=0: expected 1 error, got %v", errs)
	}
}

func TestValidateGitSinkWarnings_nilAndNonGit(t *testing.T) {
	t.Parallel()

	// nil spec → nil (line 109-111)
	if warns := ValidateGitSinkWarnings(nil); warns != nil {
		t.Fatalf("nil spec: %v", warns)
	}

	// non-git type → nil
	if warns := ValidateGitSinkWarnings(&kollectdevv1alpha1.KollectSinkSpec{Type: "s3"}); warns != nil {
		t.Fatalf("non-git: %v", warns)
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
