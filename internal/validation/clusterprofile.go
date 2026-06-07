// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateClusterProfile checks spec, paths, and security policy for a KollectClusterProfile.
func ValidateClusterProfile(profile *kollectdevv1alpha1.KollectClusterProfile) field.ErrorList {
	if profile == nil {
		return nil
	}

	proxy := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: profile.ObjectMeta,
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK:  profile.Spec.TargetGVK,
			Attributes: profile.Spec.Attributes,
			Export:     profile.Spec.Export,
			Metrics:    profile.Spec.Metrics,
		},
	}

	return ValidateProfile(proxy)
}

// ClusterProfileWarnings returns admission warnings for paths that are valid but discouraged.
func ClusterProfileWarnings(profile *kollectdevv1alpha1.KollectClusterProfile) []string {
	proxy := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: profile.ObjectMeta,
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK:  profile.Spec.TargetGVK,
			Attributes: profile.Spec.Attributes,
			Export:     profile.Spec.Export,
			Metrics:    profile.Spec.Metrics,
		},
	}

	return ProfileWarnings(proxy)
}

// ClusterProfileInvalid formats a validation failure for admission.
func ClusterProfileInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectClusterProfile %q is invalid: %s", name, formatErrors(errs))
}
