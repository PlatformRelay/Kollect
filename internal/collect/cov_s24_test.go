// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
)

// COV-90-S24: compileMatchPolicy and ValidateAttributePath error branches.

func TestCompileMatchPolicy_empty(t *testing.T) {
	t.Parallel()

	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := compileMatchPolicy(env, "  "); err == nil {
		t.Fatal("expected error for empty matchPolicy expression")
	}
}

func TestValidateAttributePath_invalidJSONPath(t *testing.T) {
	t.Parallel()

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateAttributePath(extractor, "$.[invalid")
	if err == nil {
		t.Fatal("expected JSONPath parse error")
	}
	if !strings.Contains(err.Error(), "parse JSONPath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttributePath_invalidCELCompile(t *testing.T) {
	t.Parallel()

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateAttributePath(extractor, "cel:1 +")
	if err == nil {
		t.Fatal("expected CEL compile error")
	}
	if !strings.Contains(err.Error(), "compile CEL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMatchPolicyExpression_compileError(t *testing.T) {
	t.Parallel()

	if err := ValidateMatchPolicyExpression("object."); err == nil {
		t.Fatal("expected compile error for invalid matchPolicy")
	}
}
