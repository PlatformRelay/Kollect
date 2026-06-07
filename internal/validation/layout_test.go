// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateLayoutSpec_Valid(t *testing.T) {
	t.Parallel()
	maxSeg := int32(40)
	l := &kollectdevv1alpha1.LayoutSpec{
		Mode:         kollectdevv1alpha1.LayoutModePerResource,
		Content:      kollectdevv1alpha1.LayoutContentManifest,
		PathTemplate: "{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}",
		Index:        &kollectdevv1alpha1.LayoutIndexSpec{PathTemplate: "inventory/{namespace}/{name}{extension}"},
		Filename: &kollectdevv1alpha1.LayoutFilenameSpec{
			GroupInPath: kollectdevv1alpha1.LayoutGroupInPathAuto, MaxSegmentLength: &maxSeg,
		},
	}
	if errs := ValidateLayoutSpec(l, field.NewPath("spec").Child("layout")); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateLayoutSpec_Invalid(t *testing.T) {
	t.Parallel()
	bad := int32(0)
	l := &kollectdevv1alpha1.LayoutSpec{
		Mode:         "weird",
		Content:      "nope",
		PathTemplate: "{cluster}/{bogus}/{sourceName}",
		Filename:     &kollectdevv1alpha1.LayoutFilenameSpec{GroupInPath: "sometimes", MaxSegmentLength: &bad},
	}
	errs := ValidateLayoutSpec(l, field.NewPath("spec").Child("layout"))
	if len(errs) < 4 {
		t.Fatalf("expected multiple errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateSnapshotSinkSpec_LayoutForbiddenOnObjectStore(t *testing.T) {
	t.Parallel()
	spec := &kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeS3,
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			Layout: &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
		},
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{},
	}
	errs := ValidateSnapshotSinkSpec(spec)
	if !hasFieldError(errs, "spec.layout") {
		t.Fatalf("expected forbidden layout error, got %v", errs)
	}
}

func TestValidateSnapshotSinkSpec_LayoutAllowedOnGit(t *testing.T) {
	t.Parallel()
	spec := &kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			Endpoint: "https://git.example.com/acme/inv.git",
			Layout:   &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
		},
		Git: &kollectdevv1alpha1.GitSpec{},
	}
	if hasFieldError(ValidateSnapshotSinkSpec(spec), "spec.layout") {
		t.Fatalf("layout must be allowed for git")
	}
}

func TestValidateSinkFormatCapability_GitAcceptsYAML(t *testing.T) {
	t.Parallel()
	path := field.NewPath("spec").Child("serialization").Child("format")
	if errs := ValidateSinkFormatCapability(kollectdevv1alpha1.SnapshotSinkTypeGit, "yaml", path); len(errs) > 0 {
		t.Fatalf("git should accept yaml: %v", errs)
	}
	if errs := ValidateSinkFormatCapability(kollectdevv1alpha1.SnapshotSinkTypeGit, "parquet", path); len(errs) == 0 {
		t.Fatal("git should reject parquet")
	}
	if errs := ValidateSinkFormatCapability(kollectdevv1alpha1.SnapshotSinkTypeS3, "yaml", path); len(errs) == 0 {
		t.Fatal("s3 should reject yaml")
	}
}

func TestLayoutWarnings(t *testing.T) {
	t.Parallel()
	warns := LayoutWarnings(&kollectdevv1alpha1.LayoutSpec{
		Mode: kollectdevv1alpha1.LayoutModePerResource, Content: kollectdevv1alpha1.LayoutContentManifest,
	})
	joined := strings.Join(warns, "\n")
	if !strings.Contains(joined, "manifest") || !strings.Contains(joined, "prune") {
		t.Fatalf("warnings = %v", warns)
	}
}

func hasFieldError(errs field.ErrorList, path string) bool {
	for _, e := range errs {
		if strings.HasPrefix(e.Field, path) {
			return true
		}
	}

	return false
}
