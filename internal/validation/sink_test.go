// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateSinkSpec_requiresType(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkSpec(&kollectdevv1alpha1.KollectSinkSpec{})
	if len(errs) != 1 {
		t.Fatalf("expected type required, got %d: %v", len(errs), errs)
	}
}

func TestValidateSinkSpec_acceptsRegisteredTypes(t *testing.T) {
	t.Parallel()

	for _, sinkType := range validSinkTypes {
		errs := ValidateSinkSpec(&kollectdevv1alpha1.KollectSinkSpec{Type: sinkType})
		if len(errs) != 0 {
			t.Fatalf("type %q: unexpected errors: %v", sinkType, errs)
		}
	}
}

func TestValidateSinkSpec_rejectsInvalidPathTemplate(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type:         kollectdevv1alpha1.SinkTypeS3,
		PathTemplate: "{cluster}/{name}.json",
	})
	if len(errs) != 1 {
		t.Fatalf("expected pathTemplate error, got %d: %v", len(errs), errs)
	}
}

func TestValidateSinkSpec_rejectsUnknownType(t *testing.T) {
	t.Parallel()

	errs := ValidateSinkSpec(&kollectdevv1alpha1.KollectSinkSpec{Type: "minio"})
	if len(errs) != 1 {
		t.Fatalf("expected unsupported type error, got %d: %v", len(errs), errs)
	}
}

func TestValidateSinkSpec_nilIsNoop(t *testing.T) {
	t.Parallel()

	if errs := ValidateSinkSpec(nil); errs != nil {
		t.Fatalf("expected nil errors for nil spec, got: %v", errs)
	}
}

func TestSinkInvalid_formats(t *testing.T) {
	t.Parallel()

	errs := field.ErrorList{
		field.Required(field.NewPath("spec").Child("type"), "type is required"),
	}
	err := SinkInvalid("my-sink", errs)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "KollectSink") || !strings.Contains(err.Error(), "my-sink") {
		t.Fatalf("unexpected error format: %v", err)
	}
}
