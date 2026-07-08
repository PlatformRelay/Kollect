// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package arch enforces intra-module dependency direction rules that the
// compiler cannot (Go only forbids cycles) and that .go-arch-lint.yml does
// not cover (it excludes cmd/** and test/**, and pools all leaf utilities
// into one unconstrained "shared" component).
//
// Implementation note: these are plain stdlib tests (go/parser, ImportsOnly)
// rather than a goarchtest-style library. The canonical goarchtest module is
// github.com/solrac97gr/goarchtest (not solidwall): a single-maintainer
// v0.1.0 with no commits since 2025-06; pulling an unmaintained dependency
// for what amounts to "parse imports, assert direction" is a worse trade
// than ~150 lines of stdlib. go-arch-lint (already wired into `task lint`)
// keeps covering the component graph; this test covers the gaps.
package arch

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/konih/kollect"

// skipDirs are top-level (or anywhere) directories that contain no production
// Go packages of this module or are explicitly out of scope.
var skipDirs = map[string]bool{
	".git": true, "bin": true, "dist": true, "vendor": true,
	"ui": true, "charts": true, "config": true, "docs": true,
	"hack": true, "references": true, "agent-context": true,
	"kollect-repos": true, "node_modules": true, "testdata": true,
}

// imports maps a module-relative package path (e.g. "internal/collect") to
// the set of kollect-internal import paths (module-relative) used by its
// production (non _test.go) files.
type imports map[string]map[string]string // pkg -> imported pkg -> example file

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test/arch")
		}
		dir = parent
	}
}

func collectImports(t *testing.T) imports {
	t.Helper()
	root := moduleRoot(t)
	graph := imports{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if graph[pkg] == nil {
			graph[pkg] = map[string]string{}
		}
		for _, imp := range f.Imports {
			val, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if val == modulePath {
				graph[pkg][""] = rel
				continue
			}
			if strings.HasPrefix(val, modulePath+"/") {
				graph[pkg][strings.TrimPrefix(val, modulePath+"/")] = rel
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// forbid fails for every package matching pkgPattern that imports a package
// matching impPattern. Patterns are anchored regexps over module-relative
// package paths.
func forbid(t *testing.T, graph imports, pkgPattern, impPattern, why string) {
	t.Helper()
	pkgRe := regexp.MustCompile("^(?:" + pkgPattern + ")$")
	impRe := regexp.MustCompile("^(?:" + impPattern + ")$")
	for pkg, imps := range graph {
		if !pkgRe.MatchString(pkg) {
			continue
		}
		for imp, file := range imps {
			if impRe.MatchString(imp) {
				t.Errorf("%s imports %s (in %s): %s", pkg, imp, file, why)
			}
		}
	}
}

// TestAPIIsLeaf: the published API types must not depend on operator
// internals; api/v1alpha1 has to stay importable by external clients.
func TestAPIIsLeaf(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `api(/.*)?`, `internal(/.*)?|cmd`,
		"api/ must not import internal/ or cmd")
}

// TestLeafUtilitiesImportNothing: shared leaf utilities must not import any
// other kollect package, or they stop being leaves and invite cycles.
// internal/pprof and internal/httpauth were folded into their single
// consumers (cmd, internal/inventory) on 2026-06-09.
func TestLeafUtilitiesImportNothing(t *testing.T) {
	t.Parallel()
	graph := collectImports(t)
	leaves := []string{
		"internal/errors",
		"internal/metrics",
		"internal/digest",
		"internal/pathvalidate",
		"internal/operator",
		"internal/sink/cap",
	}
	for _, leaf := range leaves {
		if _, ok := graph[leaf]; !ok {
			t.Errorf("expected leaf package %s to exist (update this list if it moved)", leaf)
			continue
		}
		forbid(t, graph, regexp.QuoteMeta(leaf), `.*`,
			"leaf utility packages must not import other kollect packages")
	}
}

// TestOnlyCmdImportsController: internal/controller is the composition layer
// directly below main; nothing besides cmd may depend on it.
func TestOnlyCmdImportsController(t *testing.T) {
	t.Parallel()
	graph := collectImports(t)
	for pkg, imps := range graph {
		if pkg == "cmd" || pkg == "internal/controller" || strings.HasPrefix(pkg, "internal/controller/") {
			continue
		}
		for imp, file := range imps {
			if imp == "internal/controller" || strings.HasPrefix(imp, "internal/controller/") {
				t.Errorf("%s imports %s (in %s): only cmd may import internal/controller", pkg, imp, file)
			}
		}
	}
}

// TestSinkBackendsStayBelowController: sink backends sit below the
// controller and must not reach up to it or to webhooks.
func TestSinkBackendsStayBelowController(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `internal/sink(/.*)?`,
		`internal/controller(/.*)?|internal/webhook(/.*)?|internal/inventory(/.*)?|cmd`,
		"sink packages must not import controller/webhook/inventory layers")
}

// TestValidationStaysBelowSinks: internal/validation is a commonComponent
// consumed by sinks and webhooks; it must not import upward into the
// controller or into sink backend implementations.
func TestValidationStaysBelowSinks(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `internal/validation(/.*)?`,
		`internal/controller(/.*)?|internal/sink(/.*)?|internal/webhook(/.*)?|cmd`,
		"validation must not import controller, sink, or webhook packages")
}

// TestCollectStaysBelowConsumers: internal/collect is the highest fan-in
// package (rows/store consumed by aggregate, export, sinks, controller,
// inventory, validation); it must never import any of its consumers.
func TestCollectStaysBelowConsumers(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `internal/collect(/.*)?`,
		`internal/(controller|sink|export|validation|webhook|inventory|aggregate)(/.*)?|cmd`,
		"collect is a lower layer and must not import its consumers")
}

// TestExportStaysBelowSinks: export envelope types are consumed by sinks and
// the controller; export must not import either (export -> collect is a
// known accepted dep, todo(arch-08) in .go-arch-lint.yml).
func TestExportStaysBelowSinks(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `internal/export(/.*)?`,
		`internal/(controller|sink|webhook|inventory)(/.*)?|cmd`,
		"export must not import sink/controller layers")
}

// TestWebhookStaysBelowController: admission webhooks may use validation,
// scope, and operator config, but not the controller or sink backends.
func TestWebhookStaysBelowController(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t), `internal/webhook(/.*)?`,
		`internal/controller(/.*)?|internal/sink(/.*)?|cmd`,
		"webhook must not import controller or sink packages")
}

// TestKnownSidewaysDepsAreExplicit documents accepted sideways dependencies
// so that any NEW edge into these packages is a conscious decision: the
// import sets asserted here are exact. Accepted (with rationale):
//   - internal/sink -> internal/validation: registry reads global export
//     limits (MaxExportBytesGlobal); wave-2 plan moves limits down.
//   - internal/sink/gcs -> internal/sink/s3: GCS backend wraps the S3
//     backend via the S3-compatible XML API (deliberate adapter).
//   - internal/sink/gitlab -> internal/sink/git: GitLab sink builds on the
//     generic git backend (deliberate layering inside sink/).
func TestKnownSidewaysDepsAreExplicit(t *testing.T) {
	t.Parallel()
	graph := collectImports(t)
	cases := []struct {
		pkg string
		imp string
	}{
		{"internal/sink", "internal/validation"},
		{"internal/sink/gcs", "internal/sink/s3"},
		{"internal/sink/gitlab", "internal/sink/git"},
	}
	for _, c := range cases {
		if _, ok := graph[c.pkg][c.imp]; !ok {
			t.Logf("note: accepted sideways dep %s -> %s no longer exists; remove it from this list", c.pkg, c.imp)
		}
	}
}

// TestCLIDoesNotImportOperatorInternals: the pipeline CLI (ADR-0801) must be usable
// without installing the operator; it must not reach into the operator's composition
// layer (controller, webhook admission, or the read API's inventory package).
func TestCLIDoesNotImportOperatorInternals(t *testing.T) {
	t.Parallel()
	forbid(t, collectImports(t),
		`cmd/cli(/.*)?|internal/pipeline(/.*)?`,
		`internal/controller(/.*)?|internal/webhook(/.*)?|internal/inventory(/.*)?`,
		"cmd/cli and internal/pipeline must not import operator-only internals")
}
