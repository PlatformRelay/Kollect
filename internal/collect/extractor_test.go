// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestExtractor_Extract(t *testing.T) {
	t.Parallel()

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor() error = %v", err)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
			"labels": map[string]any{
				"app": "kollect",
			},
		},
		"data": map[string]any{
			"key": "value",
		},
	}}

	tests := []struct {
		name    string
		attrs   []kollectdevv1alpha1.AttributeSpec
		want    map[string]any
		wantErr bool
	}{
		{
			name: "jsonpath metadata name",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "name", Path: "{.metadata.name}"},
			},
			want: map[string]any{"name": "demo"},
		},
		{
			name: "jsonpath dollar prefix",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "namespace", Path: "$.metadata.namespace"},
			},
			want: map[string]any{"namespace": "default"},
		},
		{
			name: "cel label check",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "hasApp", Path: "cel:'app' in object.metadata.labels"},
			},
			want: map[string]any{"hasApp": true},
		},
		{
			name: "cel string concat",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "fqn", Path: "cel:object.metadata.namespace + '/' + object.metadata.name"},
			},
			want: map[string]any{"fqn": "default/demo"},
		},
		{
			name: "optional missing path skipped",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "missing", Path: "{.metadata.annotations.missing}", Optional: true},
				{Name: "name", Path: "{.metadata.name}"},
			},
			want: map[string]any{"name": "demo"},
		},
		{
			name: "required missing path returns nil",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "missing", Path: "{.metadata.annotations.missing}"},
			},
			want: map[string]any{"missing": nil},
		},
		{
			name: "invalid jsonpath errors",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "bad", Path: "{.metadata.name"},
			},
			wantErr: true,
		},
		{
			name: "invalid cel errors",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "bad", Path: "cel:object..metadata.name"},
			},
			wantErr: true,
		},
		{
			name: "empty path errors",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "empty", Path: "   "},
			},
			wantErr: true,
		},
	}

	deployment := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "app", "image": "app:1.0"},
						map[string]any{"name": "sidecar", "image": "sidecar:2.0"},
					},
				},
			},
		},
	}}

	deploymentTests := []struct {
		name    string
		attrs   []kollectdevv1alpha1.AttributeSpec
		want    map[string]any
		wantErr bool
	}{
		{
			name: "jsonpath single container index",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "image", Path: "$.spec.template.spec.containers[0].image", Type: "string"},
			},
			want: map[string]any{"image": "app:1.0"},
		},
		{
			name: "jsonpath all containers wildcard",
			attrs: []kollectdevv1alpha1.AttributeSpec{
				{Name: "images", Path: "$.spec.template.spec.containers[*].image", Type: "list"},
			},
			want: map[string]any{"images": []any{"app:1.0", "sidecar:2.0"}},
		},
	}

	for _, tt := range deploymentTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, extractErr := extractor.Extract(deployment, tt.attrs)
			if (extractErr != nil) != tt.wantErr {
				t.Fatalf("Extract() error = %v, wantErr %v", extractErr, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			for key, wantVal := range tt.want {
				gotVal, ok := got[key]
				if !ok {
					t.Fatalf("Extract() missing key %q", key)
				}

				switch wantSlice := wantVal.(type) {
				case []any:
					gotSlice, ok := gotVal.([]any)
					if !ok {
						t.Fatalf("Extract()[%q] = %T, want []any", key, gotVal)
					}
					if len(gotSlice) != len(wantSlice) {
						t.Fatalf("Extract()[%q] len = %d, want %d", key, len(gotSlice), len(wantSlice))
					}
					for i := range wantSlice {
						if gotSlice[i] != wantSlice[i] {
							t.Fatalf("Extract()[%q][%d] = %v, want %v", key, i, gotSlice[i], wantSlice[i])
						}
					}
				default:
					if gotVal != wantVal {
						t.Fatalf("Extract()[%q] = %v, want %v", key, gotVal, wantVal)
					}
				}
			}
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, extractErr := extractor.Extract(obj, tt.attrs)
			if (extractErr != nil) != tt.wantErr {
				t.Fatalf("Extract() error = %v, wantErr %v", extractErr, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			for key, wantVal := range tt.want {
				if gotVal, ok := got[key]; !ok {
					t.Fatalf("Extract() missing key %q", key)
				} else if gotVal != wantVal {
					t.Fatalf("Extract()[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}
