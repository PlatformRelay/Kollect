// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestRegisterIsOnlyCalledThroughRegisterOnce keeps `go test -count=N` (N > 1) safe for this
// package. Register() adds the collectors to controller-runtime's process-global registry, so a
// second call in one test binary panics on the duplicate registration (METRICS-COUNT2-PANIC). Every
// test must therefore go through registerOnce.Do(Register), which passes Register as a value; a
// direct call expression `Register()` in any test file of this package is the defect.
func TestRegisterIsOnlyCalledThroughRegisterOnce(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob test files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("found no test files -- nothing was checked")
	}

	fset := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, isIdent := call.Fun.(*ast.Ident); isIdent && ident.Name == "Register" {
				t.Errorf("%s: direct Register() call; use registerOnce.Do(Register) so -count=2 does not panic",
					fset.Position(call.Pos()))
			}

			return true
		})
	}
}
