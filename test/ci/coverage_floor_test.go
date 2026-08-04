// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package ci guards CI / Task / Codecov coverage-floor knobs stay aligned.
// COV-90-S08 ratchets the floor; a drift between Taskfile, workflow, Codecov,
// and hack/coverage.sh would let a bare local run silently enforce a stale
// fallback (historically 65) while CI used a different gate.
package ci

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// expectedFloor is the enforced COVERAGE_MIN for Track A (COV-90-S08 step 1).
const expectedFloor = "87"

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
			t.Fatal("go.mod not found above test/ci")
		}
		dir = parent
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	//nolint:gosec // G304: rel is a fixed repo-relative path from the test table.
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestCoverageFloorKnobsAligned(t *testing.T) {
	root := moduleRoot(t)
	wantQuoted := `"` + expectedFloor + `"`
	wantPct := expectedFloor + "%"
	wantFallback := `${COVERAGE_MIN:-` + expectedFloor + `}`

	cases := []struct {
		name string
		rel  string
		re   *regexp.Regexp
		want string
		min  int // minimum matches required
	}{
		{
			name: "Taskfile env+vars COVERAGE_MIN",
			rel:  "Taskfile.yml",
			re:   regexp.MustCompile(`(?m)^\s*COVERAGE_MIN:\s*"(\d+)"\s*$`),
			want: expectedFloor,
			min:  2,
		},
		{
			name: "CI workflow COVERAGE_MIN",
			rel:  ".github/workflows/ci.yaml",
			re:   regexp.MustCompile(`(?m)^\s*COVERAGE_MIN:\s*"(\d+)"\s*$`),
			want: expectedFloor,
			min:  1,
		},
		{
			name: "codecov project target",
			rel:  "codecov.yml",
			re:   regexp.MustCompile(`(?m)^\s*target:\s*(\d+%)\s*$`),
			want: wantPct,
			min:  1,
		},
		{
			name: "coverage.sh fallback",
			rel:  "hack/coverage.sh",
			re:   regexp.MustCompile(`COVERAGE_MIN:-(\d+)`),
			want: expectedFloor,
			min:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := readFile(t, root, tc.rel)
			matches := tc.re.FindAllStringSubmatch(body, -1)
			if len(matches) < tc.min {
				t.Fatalf("%s: want ≥%d matches for floor %s, got %d\nbody snippet around COVERAGE/target:\n%s",
					tc.rel, tc.min, expectedFloor, len(matches), snippetFloor(body))
			}
			for _, m := range matches {
				got := m[1]
				if got != tc.want {
					t.Errorf("%s: got %q, want %q (full match %q)", tc.rel, got, tc.want, m[0])
				}
			}
		})
	}

	// Explicit string presence checks catch YAML quoting / comment-only drift.
	task := readFile(t, root, "Taskfile.yml")
	if !strings.Contains(task, "COVERAGE_MIN: "+wantQuoted) {
		t.Errorf("Taskfile.yml missing COVERAGE_MIN: %s", wantQuoted)
	}
	ci := readFile(t, root, ".github/workflows/ci.yaml")
	if !strings.Contains(ci, "COVERAGE_MIN: "+wantQuoted) {
		t.Errorf("ci.yaml missing COVERAGE_MIN: %s", wantQuoted)
	}
	cov := readFile(t, root, "codecov.yml")
	if !strings.Contains(cov, "target: "+wantPct) {
		t.Errorf("codecov.yml missing target: %s", wantPct)
	}
	sh := readFile(t, root, "hack/coverage.sh")
	if !strings.Contains(sh, wantFallback) {
		t.Errorf("hack/coverage.sh missing fallback %s", wantFallback)
	}
}

func snippetFloor(body string) string {
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "COVERAGE_MIN") || strings.Contains(line, "target:") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	s := b.String()
	if s == "" {
		return "(no COVERAGE_MIN/target lines)"
	}
	return s
}
