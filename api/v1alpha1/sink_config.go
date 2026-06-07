// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "strings"

// EffectiveSerializationFormat returns the on-wire format for a normalized sink spec (ADR-0416).
func EffectiveSerializationFormat(spec *KollectSinkSpec) string {
	if spec == nil {
		return SerializationFormatJSON
	}

	if spec.Serialization != nil {
		if f := strings.ToLower(strings.TrimSpace(spec.Serialization.Format)); f != "" {
			return f
		}
	}

	if spec.ObjectStore != nil {
		if f := strings.ToLower(strings.TrimSpace(spec.ObjectStore.Format)); f != "" {
			return f
		}
	}

	return SerializationFormatJSON
}

// EffectiveProvisioningMode returns the provisioning mode for a normalized sink spec (ADR-0416).
func EffectiveProvisioningMode(spec *KollectSinkSpec) string {
	if spec == nil || spec.Provisioning == nil {
		return ProvisioningModeEnsure
	}

	mode := strings.ToLower(strings.TrimSpace(spec.Provisioning.Mode))
	if mode == ProvisioningModeExisting {
		return ProvisioningModeExisting
	}

	return ProvisioningModeEnsure
}

// EffectiveSerializationFormatFromCommon resolves format from common fields and optional object store block.
func EffectiveSerializationFormatFromCommon(common *SinkCommonFields, objectStore *ObjectStoreSpec) string {
	spec := &KollectSinkSpec{ObjectStore: objectStore}
	if common != nil {
		spec.Serialization = common.Serialization
	}

	return EffectiveSerializationFormat(spec)
}

// EffectiveProvisioningModeFromCommon resolves provisioning mode from common fields.
func EffectiveProvisioningModeFromCommon(common *SinkCommonFields) string {
	if common == nil {
		return ProvisioningModeEnsure
	}

	return EffectiveProvisioningMode(&KollectSinkSpec{Provisioning: common.Provisioning})
}

// PreviewEnabled reports whether the sink should render status.preview (ADR-0416).
func PreviewEnabled(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	v, ok := annotations[AnnotationPreview]
	return ok && strings.EqualFold(strings.TrimSpace(v), "true")
}
