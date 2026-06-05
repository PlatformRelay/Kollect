// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestClusterScopeInvalid(t *testing.T) {
	t.Parallel()

	err := ClusterScopeInvalid("platform", k8sfield.ErrorList{
		k8sfield.Required(k8sfield.NewPath("spec").Child("allowedGVKs"), "required"),
	})
	if err == nil || !strings.Contains(err.Error(), "KollectClusterScope") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateProfileMetricsEdgeCases(t *testing.T) {
	t.Parallel()

	base := kollectdevv1alpha1.KollectProfileSpec{
		TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Pod"},
		Attributes: []kollectdevv1alpha1.AttributeSpec{
			{Name: "ready", Path: "$.status.ready", Type: "int"},
			{Name: "phase", Path: "$.status.phase", Type: "string"},
		},
	}

	spec := base
	spec.Metrics = []kollectdevv1alpha1.MetricSpec{
		{Name: "", Path: "ready"},
		{Name: "dup", Path: "ready"},
		{Name: "dup", Path: "phase"},
		{Name: "labeled", Path: "ready", Labels: []string{"", "missing", "phase", "ready", "phase", "ready"}},
	}
	errs := ValidateProfileSpec(&spec)
	if len(errs) < 4 {
		t.Fatalf("expected multiple metric validation errors, got %d: %v", len(errs), errs)
	}
}

func TestPathTargetsSecretDataBranches(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"$.metadata.name":          false,
		"$.data.token":             true,
		"cel:object.data.release":  true,
		"cel:object.metadata.name": false,
		"":                         false,
	}

	for path, want := range cases {
		if got := pathTargetsSecretData(path); got != want {
			t.Fatalf("pathTargetsSecretData(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSplitCELPath(t *testing.T) {
	t.Parallel()

	got := splitCELPath("object.metadata.labels['app']")
	if len(got) == 0 || got[0] != "metadata" {
		t.Fatalf("splitCELPath = %#v", got)
	}

	if splitCELPath("") != nil {
		t.Fatal("empty expression should return nil")
	}
}
