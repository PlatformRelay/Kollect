// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const celPrefix = "cel:"

// Extractor evaluates CEL and JSONPath attribute paths against unstructured objects.
type Extractor struct {
	celEnv *cel.Env
}

// NewExtractor builds an Extractor with a CEL environment bound to the `object` variable.
func NewExtractor() (*Extractor, error) {
	env, err := cel.NewEnv(
		cel.Variable("object", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}

	return &Extractor{celEnv: env}, nil
}

// Extract evaluates all attributes against obj and returns a map keyed by attribute name.
func (e *Extractor) Extract(
	obj *unstructured.Unstructured,
	attrs []kollectdevv1alpha1.AttributeSpec,
) (map[string]any, error) {
	results := make(map[string]any, len(attrs))

	for _, attr := range attrs {
		val, err := e.extractOne(obj, attr.Path)
		if err != nil {
			if attr.Optional {
				continue
			}

			return nil, fmt.Errorf("attribute %q: %w", attr.Name, err)
		}

		if val == nil && attr.Optional {
			continue
		}

		results[attr.Name] = val
	}

	return results, nil
}

func (e *Extractor) extractOne(obj *unstructured.Unstructured, path string) (any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	if strings.HasPrefix(path, celPrefix) {
		return e.evalCEL(obj, strings.TrimPrefix(path, celPrefix))
	}

	if strings.HasPrefix(path, HelmReleasePathPrefix) {
		return extractHelmReleaseField(obj, path)
	}

	return evalJSONPath(obj.Object, normalizeJSONPath(path))
}

func normalizeJSONPath(path string) string {
	if strings.HasPrefix(path, "$.") {
		return "{" + path[1:] + "}"
	}

	return path
}

func evalJSONPath(obj map[string]any, path string) (any, error) {
	jp := jsonpath.New("extract")
	if err := jp.Parse(path); err != nil {
		return nil, fmt.Errorf("parse JSONPath: %w", err)
	}

	results, err := jp.FindResults(obj)
	if err != nil {
		if isJSONPathNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("eval JSONPath: %w", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return nil, nil
	}

	if len(results[0]) == 1 {
		return results[0][0].Interface(), nil
	}

	vals := make([]any, len(results[0]))
	for i, result := range results[0] {
		vals[i] = result.Interface()
	}

	return vals, nil
}

func (e *Extractor) evalCEL(obj *unstructured.Unstructured, expr string) (any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty CEL expression")
	}

	ast, issues := e.celEnv.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile CEL: %w", issues.Err())
	}

	prg, err := e.celEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("build CEL program: %w", err)
	}

	out, _, err := prg.Eval(map[string]any{
		"object": obj.Object,
	})
	if err != nil {
		return nil, fmt.Errorf("eval CEL: %w", err)
	}

	return celValueToGo(out), nil
}

func celValueToGo(val ref.Val) any {
	if val == types.NullValue {
		return nil
	}

	return val.Value()
}

func isJSONPathNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "is not found")
}
