// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/objectstore"
)

var validSinkTypes = []string{
	kollectdevv1alpha1.SinkTypeGit,
	kollectdevv1alpha1.SinkTypeGitLab,
	kollectdevv1alpha1.SinkTypeS3,
	kollectdevv1alpha1.SinkTypeGCS,
	kollectdevv1alpha1.SinkTypePostgres,
	kollectdevv1alpha1.SinkTypeKafka,
	kollectdevv1alpha1.SinkTypeNats,
}

// ValidateSinkSpec checks KollectSink spec fields.
func ValidateSinkSpec(spec *kollectdevv1alpha1.KollectSinkSpec) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList

	typePath := field.NewPath("spec").Child("type")
	sinkType := strings.TrimSpace(spec.Type)
	if sinkType == "" {
		allErrs = append(allErrs, field.Required(typePath, "type is required"))

		return allErrs
	}

	for _, allowed := range validSinkTypes {
		if sinkType == allowed {
			if err := objectstore.ValidatePathTemplate(spec.PathTemplate); err != nil {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("pathTemplate"),
					spec.PathTemplate,
					err.Error(),
				))
			}

			return allErrs
		}
	}

	allErrs = append(allErrs, field.NotSupported(typePath, sinkType, validSinkTypes))

	return allErrs
}

// SinkInvalid formats a validation failure for admission.
func SinkInvalid(name string, errs field.ErrorList) error {
	return fmt.Errorf("KollectSink %q is invalid: %s", name, formatErrors(errs))
}
