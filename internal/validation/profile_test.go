// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateProfileSecretDataPaths(t *testing.T) {
	t.Parallel()

	secretGVK := kollectdevv1alpha1.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}

	tests := []struct {
		name        string
		annotations map[string]string
		path        string
		wantErr     bool
	}{
		{
			name:    "deployment path on secret target allowed without annotation",
			path:    "$.metadata.name",
			wantErr: false,
		},
		{
			name:    "secret data path rejected without opt-in",
			path:    "$.data.release",
			wantErr: true,
		},
		{
			name: "secret data path allowed with opt-in annotation",
			annotations: map[string]string{
				AllowSecretExtractionAnnotation: "true",
			},
			path:    "$.data.release",
			wantErr: false,
		},
		{
			name: "opt-in annotation must be true",
			annotations: map[string]string{
				AllowSecretExtractionAnnotation: "false",
			},
			path:    "$.data.release",
			wantErr: true,
		},
		{
			name:    "cel secret data path rejected without opt-in",
			path:    "cel:object.data.release",
			wantErr: true,
		},
		{
			name:    "helm summary path allowed without opt-in",
			path:    "helm:release.chartVersion",
			wantErr: false,
		},
		{
			name:    "helm config path rejected without opt-in",
			path:    "helm:release.config",
			wantErr: true,
		},
		{
			name: "helm config path allowed with opt-in annotation",
			annotations: map[string]string{
				AllowSecretExtractionAnnotation: "true",
			},
			path:    "helm:release.config",
			wantErr: false,
		},
		{
			name:    "helm manifest path rejected by path validation",
			path:    "helm:release.manifest",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: tt.annotations,
				},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: secretGVK,
					Attributes: []kollectdevv1alpha1.AttributeSpec{
						{Name: "field", Path: tt.path, Type: "string", Optional: true},
					},
				},
			}

			errs := ValidateProfile(profile)
			if tt.wantErr && len(errs) == 0 {
				t.Fatal("expected validation error, got none")
			}

			if !tt.wantErr && len(errs) > 0 {
				t.Fatalf("unexpected validation errors: %v", errs)
			}
		})
	}
}

func TestPathTargetsSecretData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "$.metadata.name", want: false},
		{path: "$.data.release", want: true},
		{path: "cel:object.data.release", want: true},
		{path: "helm:release.chartVersion", want: false},
		{path: "cel:object.status.phase", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			if got := pathTargetsSecretData(tt.path); got != tt.want {
				t.Fatalf("pathTargetsSecretData(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
