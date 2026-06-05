// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FuzzExtractJSONPath(f *testing.F) {
	f.Add(`{"metadata":{"name":"demo","namespace":"default"}}`, `{.metadata.name}`)
	f.Add(`{"metadata":{"name":"demo"}}`, `$.metadata.name`)
	f.Add(`{"spec":{"replicas":3}}`, `{.spec.replicas}`)

	extractor, err := NewExtractor()
	if err != nil {
		f.Fatalf("NewExtractor() error = %v", err)
	}

	f.Fuzz(func(t *testing.T, objJSON, path string) {
		if len(objJSON) > 65536 || len(path) > 4096 {
			t.Skip()
		}

		if !json.Valid([]byte(objJSON)) {
			return
		}

		var objMap map[string]any
		if err := json.Unmarshal([]byte(objJSON), &objMap); err != nil {
			return
		}

		obj := &unstructured.Unstructured{Object: objMap}
		_, _ = extractor.extractOne(obj, path)
	})
}

func FuzzExtractCEL(f *testing.F) {
	f.Add(`{"metadata":{"name":"demo","labels":{"app":"kollect"}}}`, `object.metadata.name`)
	f.Add(`{"metadata":{"name":"demo"}}`, `'app' in object.metadata.labels`)
	f.Add(`{"metadata":{"namespace":"ns","name":"demo"}}`, `object.metadata.namespace + '/' + object.metadata.name`)

	extractor, err := NewExtractor()
	if err != nil {
		f.Fatalf("NewExtractor() error = %v", err)
	}

	f.Fuzz(func(t *testing.T, objJSON, expr string) {
		if len(objJSON) > 65536 || len(expr) > 4096 {
			t.Skip()
		}

		if !json.Valid([]byte(objJSON)) {
			return
		}

		var objMap map[string]any
		if err := json.Unmarshal([]byte(objJSON), &objMap); err != nil {
			return
		}

		obj := &unstructured.Unstructured{Object: objMap}
		path := celPrefix + expr
		_, _ = extractor.extractOne(obj, path)
	})
}

func FuzzValidateAttributePath(f *testing.F) {
	f.Add(`{.metadata.name}`)
	f.Add(`$.metadata.namespace`)
	f.Add(`cel:object.metadata.name`)
	f.Add(`cel:'app' in object.metadata.labels`)

	extractor, err := NewExtractor()
	if err != nil {
		f.Fatalf("NewExtractor() error = %v", err)
	}

	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 4096 {
			t.Skip()
		}

		_ = ValidateAttributePath(extractor, path)
	})
}
