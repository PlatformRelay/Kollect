// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"strings"
	"testing"

	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func newTestExtractor(t *testing.T) *Extractor {
	t.Helper()

	e, err := NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	return e
}

func extractorTestObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cfg", "namespace": "default"},
		"items":      []any{},
	}}
}

// TestExtract_optionalAttributeErrorIsSkipped locks the optional contract: a failing
// optional attribute is silently omitted while required siblings still extract.
func TestExtract_optionalAttributeErrorIsSkipped(t *testing.T) {
	t.Parallel()

	e := newTestExtractor(t)

	attrs := []kollectdevv1alpha1.AttributeSpec{
		{Name: "broken", Path: "cel:object.metadata ==", Optional: true},
		{Name: "name", Path: "$.metadata.name"},
	}

	got, err := e.Extract(extractorTestObject(), attrs)
	if err != nil {
		t.Fatalf("Extract() error = %v, want optional failure swallowed", err)
	}
	if _, present := got["broken"]; present {
		t.Fatalf("broken optional attribute must be omitted, got %v", got["broken"])
	}
	if got["name"] != "cfg" {
		t.Fatalf("name = %v, want cfg", got["name"])
	}
}

func TestExtract_emptyCELExpressionErrors(t *testing.T) {
	t.Parallel()

	e := newTestExtractor(t)

	attrs := []kollectdevv1alpha1.AttributeSpec{{Name: "empty", Path: "cel:   "}}

	_, err := e.Extract(extractorTestObject(), attrs)
	if err == nil || !strings.Contains(err.Error(), "empty CEL expression") {
		t.Fatalf("err = %v, want wrapped empty-CEL error", err)
	}
	if !strings.Contains(err.Error(), `attribute "empty"`) {
		t.Fatalf("err = %v, want attribute-scoped wrapping", err)
	}
}

func TestExtract_jsonPathEvalErrorIsWrapped(t *testing.T) {
	t.Parallel()

	e := newTestExtractor(t)

	// Indexing into a scalar is an evaluation error, not a not-found miss.
	attrs := []kollectdevv1alpha1.AttributeSpec{{Name: "bad", Path: "{.metadata.name[0]}"}}

	_, err := e.Extract(extractorTestObject(), attrs)
	if err == nil || !strings.Contains(err.Error(), "eval JSONPath") {
		t.Fatalf("err = %v, want wrapped eval JSONPath error", err)
	}
}

func TestExtract_emptyRangeResultIsNil(t *testing.T) {
	t.Parallel()

	e := newTestExtractor(t)

	attrs := []kollectdevv1alpha1.AttributeSpec{{Name: "items", Path: "$.items[*]"}}

	got, err := e.Extract(extractorTestObject(), attrs)
	if err != nil {
		t.Fatalf("Extract() error = %v, want empty range to yield nil", err)
	}
	val, present := got["items"]
	if !present {
		t.Fatal("items key absent, want present with nil value for empty range")
	}
	if val != nil {
		t.Fatalf("items = %v, want nil for empty range", val)
	}
}

func TestCELValueToGo_nullValue(t *testing.T) {
	t.Parallel()

	if got := celValueToGo(types.NullValue); got != nil {
		t.Fatalf("celValueToGo(NullValue) = %v, want nil", got)
	}

	if got := celValueToGo(types.String("x")); got != "x" {
		t.Fatalf("celValueToGo(String) = %v, want x", got)
	}
}
