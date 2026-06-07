// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "testing"

func TestEffectiveSerializationFormat_precedence(t *testing.T) {
	spec := &KollectSinkSpec{
		Serialization: &SerializationSpec{Format: "parquet"},
		ObjectStore:   &ObjectStoreSpec{Format: "json"},
	}
	if got := EffectiveSerializationFormat(spec); got != SerializationFormatParquet {
		t.Fatalf("expected parquet, got %q", got)
	}
}

func TestEffectiveSerializationFormat_objectStoreFallback(t *testing.T) {
	spec := &KollectSinkSpec{ObjectStore: &ObjectStoreSpec{Format: "parquet"}}
	if got := EffectiveSerializationFormat(spec); got != SerializationFormatParquet {
		t.Fatalf("expected parquet, got %q", got)
	}
}

func TestEffectiveProvisioningMode_defaultsEnsure(t *testing.T) {
	if got := EffectiveProvisioningMode(&KollectSinkSpec{}); got != ProvisioningModeEnsure {
		t.Fatalf("expected ensure, got %q", got)
	}
}

func TestPreviewEnabled(t *testing.T) {
	if !PreviewEnabled(map[string]string{"kollect.dev/preview": "true"}) {
		t.Fatal("expected preview enabled")
	}
	if PreviewEnabled(map[string]string{"kollect.dev/preview": "false"}) {
		t.Fatal("expected preview disabled")
	}
}
