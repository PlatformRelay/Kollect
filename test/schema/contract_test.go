// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package schema

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestCRDSpecOpenAPIFragments(t *testing.T) {
	root := repoRoot(t)
	update := os.Getenv("UPDATE_GOLDEN") == "1"

	for _, tc := range DefaultCases {
		t.Run(tc.GoldenFile, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractSpecOpenAPIFragment(CRDPath(root, tc.CRDFile))
			if err != nil {
				t.Fatalf("extract: %v", err)
			}

			goldenPath := GoldenPath(root, tc.GoldenFile)
			if update {
				if mkdirErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750); mkdirErr != nil {
					t.Fatalf("mkdir golden dir: %v", mkdirErr)
				}

				if writeErr := os.WriteFile(goldenPath, got, 0o600); writeErr != nil {
					t.Fatalf("write golden: %v", writeErr)
				}

				t.Logf("updated %s", goldenPath)

				return
			}

			//nolint:gosec // G304: goldenPath is repo-relative under test/schema/golden.
			want, readErr := os.ReadFile(goldenPath)
			if readErr != nil {
				t.Fatalf(
					"read golden %q: %v (run UPDATE_GOLDEN=1 go test ./test/schema/ -run TestCRDSpecOpenAPIFragments)",
					goldenPath,
					readErr,
				)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf(
					"OpenAPI spec fragment drift for %s\n"+
						"run: UPDATE_GOLDEN=1 go test ./test/schema/ -run TestCRDSpecOpenAPIFragments",
					tc.CRDFile,
				)
			}
		})
	}
}
