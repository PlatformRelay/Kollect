// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func deploymentFixture() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
			"uid":       "bench-uid",
			"labels": map[string]any{
				"app": "kollect",
			},
		},
		"spec": map[string]any{
			"replicas": int64(3),
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "kollect"},
				},
			},
		},
		"status": map[string]any{
			"readyReplicas": int64(3),
		},
	}}
}

func BenchmarkExtract(b *testing.B) {
	extractor, err := NewExtractor()
	if err != nil {
		b.Fatalf("NewExtractor: %v", err)
	}

	obj := deploymentFixture()
	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "name", Path: "{.metadata.name}"},
		{Name: "replicas", Path: "{.spec.replicas}"},
		{Name: "ready", Path: "cel:has(object.status.readyReplicas) ? object.status.readyReplicas : 0"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := extractor.Extract(obj, attrs); err != nil {
			b.Fatal(err)
		}
	}
}
