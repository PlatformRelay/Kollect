// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestProfileWarningsJSONPathFilter(t *testing.T) {
	t.Parallel()

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "name", Path: "$.metadata.name", Type: "string"},
				{Name: "running", Path: "$.items[?(@.status.phase=='Running')].name", Type: "string"},
			},
		},
	}

	warnings := ProfileWarnings(profile)
	if len(warnings) != 1 {
		t.Fatalf("ProfileWarnings() len = %d, want 1", len(warnings))
	}

	if !strings.Contains(warnings[0], "running") {
		t.Fatalf("warning = %q, want attribute name", warnings[0])
	}
}
