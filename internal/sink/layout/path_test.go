// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"strings"
	"testing"
)

func TestSanitizeSegment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"api", 63, "api"},
		{"a/b", 63, "a-b"},
		{"a:b\\c", 63, "a-b-c"},
		{"../etc", 63, "etc"},
		{"  spaced  ", 63, "spaced"},
		{"longname", 4, "long"},
		{"....", 63, ""},
	}
	for _, c := range cases {
		if got := sanitizeSegment(c.in, c.maxLen); got != c.want {
			t.Errorf("sanitizeSegment(%q,%d) = %q, want %q", c.in, c.maxLen, got, c.want)
		}
	}
}

func TestValidateLayoutPathTemplate(t *testing.T) {
	t.Parallel()
	valid := []string{
		"",
		"{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}",
		"clusters/{cluster}/{sourceName}{extension}",
		"{namespace}/{name}{extension}",
	}
	for _, tmpl := range valid {
		if err := ValidateLayoutPathTemplate(tmpl); err != nil {
			t.Errorf("ValidateLayoutPathTemplate(%q) unexpected error: %v", tmpl, err)
		}
	}

	invalid := []struct {
		tmpl   string
		reason string
	}{
		{"{cluster}/{bogus}/{sourceName}", "unsupported placeholder"},
		{"{cluster}/{kind}", "per-resource identifier"},
		{"../{sourceName}", "must not contain"},
	}
	for _, c := range invalid {
		err := ValidateLayoutPathTemplate(c.tmpl)
		if err == nil {
			t.Errorf("ValidateLayoutPathTemplate(%q) expected error", c.tmpl)
			continue
		}
		if !strings.Contains(err.Error(), c.reason) {
			t.Errorf("ValidateLayoutPathTemplate(%q) error = %q, want contains %q", c.tmpl, err, c.reason)
		}
	}
}

func TestCleanRenderedPath_RejectsEmptyAndTraversal(t *testing.T) {
	t.Parallel()
	if _, err := cleanRenderedPath("//"); err == nil {
		t.Error("expected error for empty path")
	}
	if _, err := cleanRenderedPath("a/../b"); err == nil {
		t.Error("expected error for traversal")
	}
	got, err := cleanRenderedPath("a//b/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a/b" {
		t.Errorf("cleanRenderedPath collapsed = %q", got)
	}
}
