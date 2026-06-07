// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func deploymentGVK() kollectdevv1alpha1.GroupVersionKind {
	return kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
}

func secretGVK() kollectdevv1alpha1.GroupVersionKind {
	return kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"}
}

func TestValidateProfile_export(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile kollectdevv1alpha1.KollectProfile
		wantErr bool
	}{
		{
			name: "resource mode without attributes is allowed",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: deploymentGVK(),
					Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
				},
			},
		},
		{
			name: "attributes mode without attributes is rejected",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: deploymentGVK(),
					Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeAttributes},
				},
			},
			wantErr: true,
		},
		{
			name: "export.as colliding with attribute name is rejected",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: deploymentGVK(),
					Export: &kollectdevv1alpha1.ExportSpec{
						Mode: kollectdevv1alpha1.ExportModeResource,
						As:   "resource",
					},
					Attributes: []kollectdevv1alpha1.AttributeSpec{
						{Name: "resource", Path: "$.spec.replicas"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid RFC 6901 pointer is rejected",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: deploymentGVK(),
					Export: &kollectdevv1alpha1.ExportSpec{
						Mode:  kollectdevv1alpha1.ExportModeResource,
						Prune: &kollectdevv1alpha1.PruneSpec{JSONPointers: []string{"metadata/name"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid RFC 6901 pointer with escape accepted",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: deploymentGVK(),
					Export: &kollectdevv1alpha1.ExportSpec{
						Mode: kollectdevv1alpha1.ExportModeResource,
						Prune: &kollectdevv1alpha1.PruneSpec{
							JSONPointers: []string{"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration"},
						},
					},
				},
			},
		},
		{
			name: "secret resource export rejected without annotation",
			profile: kollectdevv1alpha1.KollectProfile{
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: secretGVK(),
					Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
				},
			},
			wantErr: true,
		},
		{
			name: "secret resource export allowed with annotation",
			profile: kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{AllowFullResourceExportAnnotation: "true"},
				},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: secretGVK(),
					Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errs := ValidateProfile(&tc.profile)
			if tc.wantErr && len(errs) == 0 {
				t.Fatalf("expected validation error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Fatalf("unexpected validation errors: %v", errs)
			}
		})
	}
}

func TestProfileWarnings_export(t *testing.T) {
	t.Parallel()

	profile := &kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: deploymentGVK(),
			Export: &kollectdevv1alpha1.ExportSpec{
				Mode: kollectdevv1alpha1.ExportModeResource,
				Prune: &kollectdevv1alpha1.PruneSpec{
					CEL:       []string{"cel:has(object.status)"},
					JSONPaths: []string{"$.spec.containers[*]"},
				},
			},
		},
	}

	warnings := ProfileWarnings(profile)
	if len(warnings) < 2 {
		t.Fatalf("expected CEL + wildcard warnings, got %v", warnings)
	}

	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "Phase 2") {
		t.Fatalf("expected Phase 2 CEL warning, got %v", warnings)
	}
}

func TestValidateClusterProfile_exportSecretGuard(t *testing.T) {
	t.Parallel()

	cp := &kollectdevv1alpha1.KollectClusterProfile{
		Spec: kollectdevv1alpha1.KollectClusterProfileSpec{
			TargetGVK: secretGVK(),
			Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
		},
	}

	if errs := ValidateClusterProfile(cp); len(errs) == 0 {
		t.Fatalf("cluster profile secret resource export should require annotation")
	}

	cp.Annotations = map[string]string{AllowFullResourceExportAnnotation: "true"}
	if errs := ValidateClusterProfile(cp); len(errs) > 0 {
		t.Fatalf("cluster profile with annotation should pass: %v", errs)
	}
}
