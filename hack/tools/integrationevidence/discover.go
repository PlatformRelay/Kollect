// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// listPackages runs `go list -json -tags=integration` for patterns in dir. A pattern that
// matches no package is an error: an empty selection would make every later check vacuous.
func listPackages(dir string, patterns []string) ([]listedPackage, error) {
	// -buildvcs=false: listing main packages would otherwise stamp VCS data, which fails in some
	// checkouts (worktrees, detached copies) and is not needed here.
	args := append([]string{"list", "-json", "-buildvcs=false", "-tags=" + integrationTag}, patterns...)

	cmd := exec.Command("go", args...) //nolint:gosec // fixed binary; patterns come from the Taskfile
	cmd.Dir = dir

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %w: %s", strings.Join(patterns, " "), err, stderr.String())
	}

	// `go list` warns on stderr, but still exits zero, when a pattern matches nothing.
	if strings.Contains(stderr.String(), "matched no packages") {
		return nil, fmt.Errorf("go list %s: %s", strings.Join(patterns, " "), strings.TrimSpace(stderr.String()))
	}

	var pkgs []listedPackage

	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}

		pkgs = append(pkgs, p)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("go list %s: no packages", strings.Join(patterns, " "))
	}

	return pkgs, nil
}

// discoverExpected returns the integration tests of pkgs: top-level TestXxx(*testing.T)
// functions in test files that build with the integration tag but not without it.
func discoverExpected(pkgs []listedPackage) (map[testID]int, error) {
	untagged := build.Default
	untagged.BuildTags = nil

	expected := map[testID]int{}

	for _, p := range pkgs {
		files := append(append([]string{}, p.TestGoFiles...), p.XTestGoFiles...)
		for _, name := range files {
			// go list already selected the file with the tag set; it is an integration file
			// exactly when the default build context, without the tag, would leave it out.
			matches, err := untagged.MatchFile(p.Dir, name)
			if err != nil {
				return nil, fmt.Errorf("match %s: %w", filepath.Join(p.Dir, name), err)
			}

			if matches {
				continue
			}

			tests, err := testFuncs(filepath.Join(p.Dir, name))
			if err != nil {
				return nil, err
			}

			for _, test := range tests {
				expected[testID{Package: p.ImportPath, Test: test}]++
			}
		}
	}

	return expected, nil
}

// testFuncs lists the functions in path that `go test` runs as tests.
func testFuncs(path string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var names []string

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !isTestName(fn.Name.Name) || !takesTestingT(fn.Type) {
			continue
		}

		names = append(names, fn.Name.Name)
	}

	return names, nil
}

// isTestName applies the go test naming rule: "Test" alone, or followed by a non-lowercase rune.
func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}

	if rest == "" {
		return true
	}

	r, _ := utf8.DecodeRuneInString(rest)

	return !unicode.IsLower(r)
}

// takesTestingT reports whether the function has exactly one parameter of type *<pkg>.T, which
// excludes TestMain(*testing.M) and helpers with other signatures.
func takesTestingT(fn *ast.FuncType) bool {
	if fn.Params == nil || len(fn.Params.List) != 1 || len(fn.Params.List[0].Names) > 1 {
		return false
	}

	star, ok := fn.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	sel, ok := star.X.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == "T"
}

// outsideSelection names the tests in all whose package is not selected: integration tests
// that the run would never execute.
func outsideSelection(all map[testID]int, selected []string) []string {
	in := make(map[string]bool, len(selected))
	for _, p := range selected {
		in[p] = true
	}

	var names []string

	for id := range all {
		if !in[id.Package] {
			names = append(names, id.String())
		}
	}

	sort.Strings(names)

	return names
}
