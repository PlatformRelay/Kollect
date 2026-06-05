// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

func validateGitSpec(spec *kollectdevv1alpha1.KollectSinkSpec) field.ErrorList {
	if spec == nil || spec.Type != kollectdevv1alpha1.SinkTypeGit || spec.Git == nil {
		return nil
	}

	var allErrs field.ErrorList
	gitPath := field.NewPath("spec").Child("git")

	if branch := strings.TrimSpace(spec.Git.Branch); branch != "" {
		if err := git.ValidateGitRef(branch); err != nil {
			allErrs = append(allErrs, field.Invalid(gitPath.Child("branch"), branch, err.Error()))
		}
	}

	switch strings.TrimSpace(spec.Git.PushPolicy) {
	case "", kollectdevv1alpha1.GitPushPolicyCommit, kollectdevv1alpha1.GitPushPolicyForcePush:
	default:
		allErrs = append(allErrs, field.NotSupported(
			gitPath.Child("pushPolicy"),
			spec.Git.PushPolicy,
			[]string{kollectdevv1alpha1.GitPushPolicyCommit, kollectdevv1alpha1.GitPushPolicyForcePush},
		))
	}

	if spec.Git.Auth != nil {
		switch strings.TrimSpace(spec.Git.Auth.Type) {
		case "", kollectdevv1alpha1.GitAuthTypeToken, kollectdevv1alpha1.GitAuthTypeSSH:
		default:
			allErrs = append(allErrs, field.NotSupported(
				gitPath.Child("auth").Child("type"),
				spec.Git.Auth.Type,
				[]string{kollectdevv1alpha1.GitAuthTypeToken, kollectdevv1alpha1.GitAuthTypeSSH},
			))
		}

		if ref := spec.Git.Auth.SecretRef; ref != nil && strings.TrimSpace(ref.Name) == "" {
			allErrs = append(allErrs, field.Required(gitPath.Child("auth").Child("secretRef").Child("name"), "name is required"))
		}
	}

	if spec.Git.CloneDepth != nil && *spec.Git.CloneDepth < 1 {
		allErrs = append(allErrs, field.Invalid(gitPath.Child("cloneDepth"), *spec.Git.CloneDepth, "must be >= 1"))
	}

	return allErrs
}

// ValidateGitSinkWarnings returns admission warnings for risky git sink settings.
func ValidateGitSinkWarnings(spec *kollectdevv1alpha1.KollectSinkSpec) []string {
	if spec == nil || spec.Type != kollectdevv1alpha1.SinkTypeGit || spec.Git == nil {
		return nil
	}

	if strings.TrimSpace(spec.Git.PushPolicy) == kollectdevv1alpha1.GitPushPolicyForcePush {
		return []string{
			fmt.Sprintf(
				"spec.git.pushPolicy=%s overwrites remote branch history; use Commit for append-only audit trails",
				kollectdevv1alpha1.GitPushPolicyForcePush,
			),
		}
	}

	return nil
}
