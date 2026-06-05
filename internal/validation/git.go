// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var safeGitRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// ValidateGitRef rejects unsafe or ambiguous git branch/tag names.
func ValidateGitRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("empty git ref")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("git ref %q must not start with '-'", ref)
	}
	if ref == "." || ref == ".." || strings.Contains(ref, "..") {
		return fmt.Errorf("git ref %q contains invalid '..'", ref)
	}
	if strings.HasPrefix(ref, ".") {
		return fmt.Errorf("git ref %q must not start with '.'", ref)
	}
	if strings.HasSuffix(ref, ".") || strings.HasSuffix(ref, ".lock") {
		return fmt.Errorf("git ref %q has invalid suffix", ref)
	}
	if ref == "@" || strings.Contains(ref, "@{") {
		return fmt.Errorf("git ref %q contains invalid '@'", ref)
	}
	if strings.HasPrefix(ref, "refs/") {
		return fmt.Errorf("git ref %q must be a short branch name", ref)
	}
	if !safeGitRefPattern.MatchString(ref) {
		return fmt.Errorf("git ref %q contains unsupported characters", ref)
	}
	return nil
}

func validateGitSpec(spec *kollectdevv1alpha1.KollectSinkSpec) field.ErrorList {
	if spec == nil || spec.Type != kollectdevv1alpha1.SinkTypeGit || spec.Git == nil {
		return nil
	}

	var allErrs field.ErrorList
	gitPath := field.NewPath("spec").Child("git")

	if branch := strings.TrimSpace(spec.Git.Branch); branch != "" {
		if err := ValidateGitRef(branch); err != nil {
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
