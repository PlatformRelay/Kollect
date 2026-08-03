// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestValidateSinkCommonConfig_RejectsUnsupportedFormat_NamesOffender asserts the
// serialization.format allowlist rejects an unknown format and the rejection NAMES
// the offending value (COV-90-S05); this drives the uncovered default branch of
// validateSerializationSpec.
func TestValidateSinkCommonConfig_RejectsUnsupportedFormat_NamesOffender(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkCommonConfig(&kollectdevv1alpha1.SinkCommonFields{
		Serialization: &kollectdevv1alpha1.SerializationSpec{Format: "avro"},
	})
	if len(errs) == 0 {
		t.Fatal("expected unsupported serialization format error")
	}
	msg := errs.ToAggregate().Error()
	if !strings.Contains(msg, "avro") {
		t.Fatalf("error must name offending format %q, got: %s", "avro", msg)
	}
	if !strings.Contains(msg, "serialization.format") {
		t.Fatalf("error must reference the serialization.format field, got: %s", msg)
	}
}

// TestValidateSinkCommonConfig_RejectsUnsupportedCompression_NamesOffender asserts the
// compression allowlist rejects an unknown codec and names it (COV-90-S05).
func TestValidateSinkCommonConfig_RejectsUnsupportedCompression_NamesOffender(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkCommonConfig(&kollectdevv1alpha1.SinkCommonFields{
		Serialization: &kollectdevv1alpha1.SerializationSpec{
			Format:      kollectdevv1alpha1.SerializationFormatJSON,
			Compression: "lz4",
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected unsupported compression error")
	}
	msg := errs.ToAggregate().Error()
	if !strings.Contains(msg, "lz4") {
		t.Fatalf("error must name offending compression %q, got: %s", "lz4", msg)
	}
	if !strings.Contains(msg, "serialization.compression") {
		t.Fatalf("error must reference the serialization.compression field, got: %s", msg)
	}
}

// TestValidateSinkCommonConfig_AcceptsSupportedFormats asserts the known-good
// format/compression pair produces no errors (guards the accept branch).
func TestValidateSinkCommonConfig_AcceptsSupportedFormats(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkCommonConfig(&kollectdevv1alpha1.SinkCommonFields{
		Serialization: &kollectdevv1alpha1.SerializationSpec{
			Format:      kollectdevv1alpha1.SerializationFormatNDJSON,
			Compression: "zstd",
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors for supported ndjson/zstd: %v", errs)
	}
}

// TestSupportedFormatsForType covers every branch of supportedFormatsForType so the
// per-sink-type serialization capability matrix is locked (COV-90-S05).
func TestSupportedFormatsForType(t *testing.T) {
	t.Parallel()

	contains := func(list []string, want string) bool {
		for _, v := range list {
			if v == want {
				return true
			}
		}
		return false
	}

	cases := []struct {
		name        string
		sinkType    string
		mustContain []string
		mustReject  []string
	}{
		{
			name:        "git honors yaml and ndjson",
			sinkType:    kollectdevv1alpha1.SnapshotSinkTypeGit,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatYAML, kollectdevv1alpha1.SerializationFormatNDJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatParquet},
		},
		{
			name:        "gitlab honors yaml and ndjson",
			sinkType:    kollectdevv1alpha1.SnapshotSinkTypeGitLab,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatYAML, kollectdevv1alpha1.SerializationFormatNDJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatCSV},
		},
		{
			name:        "s3 honors parquet and csv",
			sinkType:    kollectdevv1alpha1.SnapshotSinkTypeS3,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatParquet, kollectdevv1alpha1.SerializationFormatCSV},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatYAML},
		},
		{
			name:        "gcs honors parquet and csv",
			sinkType:    kollectdevv1alpha1.SnapshotSinkTypeGCS,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatParquet, kollectdevv1alpha1.SerializationFormatCSV},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatNDJSON},
		},
		{
			name:        "http honors ndjson",
			sinkType:    kollectdevv1alpha1.SnapshotSinkTypeHTTP,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatJSON, kollectdevv1alpha1.SerializationFormatNDJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatParquet},
		},
		{
			name:        "kafka is json only",
			sinkType:    kollectdevv1alpha1.EventSinkTypeKafka,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatNDJSON},
		},
		{
			name:        "nats is json only",
			sinkType:    kollectdevv1alpha1.EventSinkTypeNats,
			mustContain: []string{kollectdevv1alpha1.SerializationFormatJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatParquet},
		},
		{
			name:        "unknown type falls back to json only",
			sinkType:    "mystery",
			mustContain: []string{kollectdevv1alpha1.SerializationFormatJSON},
			mustReject:  []string{kollectdevv1alpha1.SerializationFormatYAML},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := supportedFormatsForType(tc.sinkType)
			for _, want := range tc.mustContain {
				if !contains(got, want) {
					t.Fatalf("%s: expected %q in supported formats %v", tc.sinkType, want, got)
				}
			}
			for _, reject := range tc.mustReject {
				if contains(got, reject) {
					t.Fatalf("%s: %q must not be a supported format, got %v", tc.sinkType, reject, got)
				}
			}
		})
	}
}

func TestValidateOptionsMap_rejectsSecretLikeKeys(t *testing.T) {
	errs := ValidateOptionsMap(map[string]string{"password": "x"}, nil)
	if len(errs) == 0 {
		t.Fatal("expected forbidden error for password key")
	}
}

func TestValidateSinkFormatCapability_rejectsParquetOnKafka(t *testing.T) {
	errs := ValidateSinkFormatCapability(
		kollectdevv1alpha1.EventSinkTypeKafka,
		kollectdevv1alpha1.SerializationFormatParquet,
		nil,
	)
	if len(errs) == 0 {
		t.Fatal("expected unsupported format error")
	}
}

func TestValidateSinkConfigWarnings_existingMode(t *testing.T) {
	warns := ValidateSinkConfigWarnings(&kollectdevv1alpha1.KollectSinkSpec{
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
	})
	if len(warns) == 0 {
		t.Fatal("expected warning for existing mode")
	}
}

func TestValidateSinkCommonConfig_RejectsInvalidProvisioningMode(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkCommonConfig(&kollectdevv1alpha1.SinkCommonFields{
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{
			Mode: "auto",
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected provisioning mode validation error")
	}
}

func TestValidateSinkCommonConfig_RejectsInvalidNamingTemplate(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkCommonConfig(&kollectdevv1alpha1.SinkCommonFields{
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{
			Mode: kollectdevv1alpha1.ProvisioningModeEnsure,
			Naming: &kollectdevv1alpha1.ProvisioningNamingSpec{
				Template: "{cluster}/{unsupported}/{name}",
			},
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected invalid naming template error")
	}
}
