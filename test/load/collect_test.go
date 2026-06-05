// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//go:build load

package load_test

import (
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

const maxLoadObjects = 2000

func TestLoadExtract(t *testing.T) {
	if os.Getenv("KOLECT_LOAD_TEST") != "1" {
		t.Skip("set KOLECT_LOAD_TEST=1 to run load tests")
	}

	extractor, err := collect.NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "name", Path: "{.metadata.name}"},
		{Name: "replicas", Path: "{.spec.replicas}"},
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name": "load-demo", "namespace": "default",
		},
		"spec": map[string]any{"replicas": int64(1)},
	}}

	start := time.Now()
	for i := 0; i < maxLoadObjects; i++ {
		if _, err := extractor.Extract(obj, attrs); err != nil {
			t.Fatalf("extract %d: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	t.Logf("extracted %d objects in %s (%.0f ops/s)",
		maxLoadObjects, elapsed, float64(maxLoadObjects)/elapsed.Seconds())
}
