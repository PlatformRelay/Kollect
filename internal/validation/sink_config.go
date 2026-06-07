// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var secretLikeOptionKey = []string{
	"password", "passwd", "secret", "token", "apikey", "api_key", "accesskey", "access_key",
	"secretkey", "secret_key", "privatekey", "private_key", "credential", "credentials",
}

// ValidateSinkCommonConfig validates cross-cutting serialization and provisioning blocks (ADR-0416).
func ValidateSinkCommonConfig(common *kollectdevv1alpha1.SinkCommonFields) field.ErrorList {
	if common == nil {
		return nil
	}

	allErrs := validateSerializationSpec(common.Serialization, field.NewPath("spec").Child("serialization"))
	allErrs = append(allErrs, validateProvisioningSpec(common.Provisioning, field.NewPath("spec").Child("provisioning"))...)

	return allErrs
}

func validateSerializationSpec(spec *kollectdevv1alpha1.SerializationSpec, path *field.Path) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	if spec.Format != "" {
		format := strings.ToLower(strings.TrimSpace(spec.Format))
		switch format {
		case kollectdevv1alpha1.SerializationFormatJSON, kollectdevv1alpha1.SerializationFormatParquet,
			kollectdevv1alpha1.SerializationFormatCSV, kollectdevv1alpha1.SerializationFormatNDJSON:
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("format"), spec.Format, []string{
				kollectdevv1alpha1.SerializationFormatJSON,
				kollectdevv1alpha1.SerializationFormatParquet,
				kollectdevv1alpha1.SerializationFormatCSV,
				kollectdevv1alpha1.SerializationFormatNDJSON,
			}))
		}
	}

	if spec.Compression != "" {
		comp := strings.ToLower(strings.TrimSpace(spec.Compression))
		switch comp {
		case "none", "gzip", "snappy", "zstd":
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("compression"), spec.Compression,
				[]string{"none", "gzip", "snappy", "zstd"}))
		}
	}

	return allErrs
}

func validateProvisioningSpec(spec *kollectdevv1alpha1.ProvisioningSpec, path *field.Path) field.ErrorList {
	if spec == nil {
		return nil
	}

	var allErrs field.ErrorList
	if spec.Mode != "" {
		mode := strings.ToLower(strings.TrimSpace(spec.Mode))
		switch mode {
		case kollectdevv1alpha1.ProvisioningModeEnsure, kollectdevv1alpha1.ProvisioningModeExisting:
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("mode"), spec.Mode, []string{
				kollectdevv1alpha1.ProvisioningModeEnsure,
				kollectdevv1alpha1.ProvisioningModeExisting,
			}))
		}
	}

	if spec.Naming != nil && strings.TrimSpace(spec.Naming.Template) != "" {
		if err := ValidatePathTemplate(spec.Naming.Template); err != nil {
			allErrs = append(allErrs, field.Invalid(path.Child("naming").Child("template"), spec.Naming.Template, err.Error()))
		}
	}

	return allErrs
}

// ValidateOptionsMap rejects secret-like keys in pass-through options (ADR-0416).
func ValidateOptionsMap(options map[string]string, path *field.Path) field.ErrorList {
	if len(options) == 0 {
		return nil
	}

	var allErrs field.ErrorList
	for key := range options {
		lower := strings.ToLower(strings.TrimSpace(key))
		for _, forbidden := range secretLikeOptionKey {
			if strings.Contains(lower, forbidden) {
				allErrs = append(allErrs, field.Forbidden(path.Key(key),
					"options must not contain secret-like keys; use secretRef"))
				break
			}
		}
	}

	return allErrs
}

// ValidateSinkFormatCapability rejects unsupported serialization.format for a sink type.
func ValidateSinkFormatCapability(sinkType, format string, path *field.Path) field.ErrorList {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == kollectdevv1alpha1.SerializationFormatJSON {
		return nil
	}

	supported := supportedFormatsForType(sinkType)
	for _, f := range supported {
		if f == format {
			return nil
		}
	}

	return field.ErrorList{field.NotSupported(path, format, supported)}
}

func supportedFormatsForType(sinkType string) []string {
	switch sinkType {
	case kollectdevv1alpha1.SnapshotSinkTypeS3, kollectdevv1alpha1.SnapshotSinkTypeGCS,
		kollectdevv1alpha1.SnapshotSinkTypeAzureBlob:
		return []string{
			kollectdevv1alpha1.SerializationFormatJSON,
			kollectdevv1alpha1.SerializationFormatParquet,
			kollectdevv1alpha1.SerializationFormatCSV,
		}
	case kollectdevv1alpha1.SnapshotSinkTypeHTTP:
		return []string{
			kollectdevv1alpha1.SerializationFormatJSON,
			kollectdevv1alpha1.SerializationFormatNDJSON,
		}
	case kollectdevv1alpha1.EventSinkTypeKafka, kollectdevv1alpha1.EventSinkTypeNats:
		return []string{kollectdevv1alpha1.SerializationFormatJSON}
	default:
		return []string{kollectdevv1alpha1.SerializationFormatJSON}
	}
}

// ValidateSinkConfigWarnings returns admission warnings for sink config implications (ADR-0416).
func ValidateSinkConfigWarnings(spec *kollectdevv1alpha1.KollectSinkSpec) []string {
	if spec == nil {
		return nil
	}

	var warns []string
	if kollectdevv1alpha1.EffectiveProvisioningMode(spec) == kollectdevv1alpha1.ProvisioningModeExisting {
		warns = append(warns, "provisioning.mode=existing: kollect will not create destination resources; preflight verifies existence")
	}

	format := kollectdevv1alpha1.EffectiveSerializationFormat(spec)
	if format == kollectdevv1alpha1.SerializationFormatParquet {
		warns = append(warns, fmt.Sprintf("serialization.format=%s: snapshot will use typed Parquet columns + JSON attributes", format))
	}

	if spec.Serialization != nil && spec.ObjectStore != nil && strings.TrimSpace(spec.ObjectStore.Format) != "" {
		warns = append(warns, "spec.serialization.format takes precedence over spec.objectStore.format")
	}

	return warns
}
