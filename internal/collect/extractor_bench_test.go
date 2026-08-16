// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// extractPoolSize is the number of *distinct* objects cycled through the hot
// path. A single reused object measures perfect cache locality and inflates the
// result; a bounded pool keeps the measurement reproducible while still walking
// different map layouts, string lengths and CEL branches on every iteration.
const extractPoolSize = 128

// deploymentPool builds a deterministic pool of Deployment-shaped objects.
// Deterministic (index-derived, no RNG) so the recorded budget is reproducible.
// Every field that the measured attribute set touches varies across the pool,
// and roughly one object in seven omits status.readyReplicas so the CEL has()
// expression exercises both branches.
func deploymentPool(n int) []*unstructured.Unstructured {
	pool := make([]*unstructured.Unstructured, n)

	for i := range pool {
		obj := map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      fmt.Sprintf("demo-%03d-%s", i, strings.Repeat("x", i%17)),
				"namespace": fmt.Sprintf("ns-%02d", i%13),
				"uid":       fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
				"labels": map[string]any{
					"app":                       "kollect",
					"app.kubernetes.io/part-of": fmt.Sprintf("tier-%d", i%5),
				},
			},
			"spec": map[string]any{
				"replicas": int64(i%9 + 1),
				"template": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{"app": "kollect"},
					},
				},
			},
		}

		if i%7 != 0 {
			obj["status"] = map[string]any{"readyReplicas": int64(i%9 + 1)}
		} else {
			obj["status"] = map[string]any{"observedGeneration": int64(i)}
		}

		pool[i] = &unstructured.Unstructured{Object: obj}
	}

	return pool
}

// extractAttrs is the measured attribute set: two JSONPath attributes and one
// CEL attribute, matching the shape of a realistic KollectInventory.
func extractAttrs() []kollectdevv1alpha1.AttributeSpec {
	return []kollectdevv1alpha1.AttributeSpec{
		{Name: "name", Path: "{.metadata.name}"},
		{Name: "replicas", Path: "{.spec.replicas}"},
		{Name: "ready", Path: "cel:has(object.status.readyReplicas) ? object.status.readyReplicas : 0"},
	}
}

// extractWorkload is the SINGLE definition of the extractor hot path. Both
// BenchmarkExtract (human-readable numbers via `task bench`) and
// TestExtractHotPathBudget (assertions that can fail) drive it, so there is
// exactly one workload to keep honest.
func extractWorkload(b *testing.B) {
	b.Helper()

	extractor, err := NewExtractor()
	if err != nil {
		b.Fatalf("NewExtractor: %v", err)
	}

	// Built outside the timed region: this measures Extract, not the fixture
	// generator.
	pool := deploymentPool(extractPoolSize)
	attrs := extractAttrs()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := extractor.Extract(pool[i%len(pool)], attrs); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtract(b *testing.B) {
	extractWorkload(b)
}
