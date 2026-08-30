// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package docs_test

import (
	"strings"
	"testing"
)

// TestExtractFencedYAMLDiscovery pins the fence scanner that feeds
// TestDocsYAMLValidatesAgainstCRDSchemas.
//
// Why this test exists: every regression in this scanner -- three rounds of them --
// was found by a human running exploits by hand against the live corpus. The gate
// validates 130 documentation pages and had no test of its own, so a scanner that
// quietly stopped finding a markup variant reported a vacuous green and no floor
// could notice: an undiscovered block is not validated, not counted, and cannot
// move minDiscoveredYAMLBlocks either.
//
// Each case pins the EXTRACTED BODIES, not merely the count, so indent stripping
// and closer matching are covered too. A case that expects no blocks is as
// load-bearing as one that expects a block: over-discovery would red the corpus
// on legitimate prose.
func TestExtractFencedYAMLDiscovery(t *testing.T) {
	t.Parallel()

	const bt = "```"

	cases := []struct {
		name string
		// why records the behaviour being pinned, so a future edit that changes it
		// has to argue with a reason rather than a bare expectation.
		why      string
		markdown string
		want     []string
	}{
		// ---- plain language tokens ----
		{
			name:     "bare yaml",
			why:      "the ordinary spelling; the corpus is almost entirely this",
			markdown: bt + "yaml\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "bare yml",
			why:      "yml is the same language to every renderer",
			markdown: bt + "yml\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "uppercase YAML",
			why:      "info strings are case-insensitive to pygments, so they must be here",
			markdown: bt + "YAML\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "trailing whitespace after the language",
			why:      "trailing spaces are invisible in a diff and must not change discovery",
			markdown: bt + "yaml   \na: 1\n" + bt,
			want:     []string{"a: 1"},
		},

		// ---- Material attribute forms ----
		{
			name:     "yaml with a title attribute",
			why:      "pymdownx renders this as YAML; an anchored pattern used to skip it",
			markdown: bt + "yaml title=\"inventory.yaml\"\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name: "yaml with a title attribute containing backticks",
			why: "REGRESSION: CommonMark forbids a backtick in a backtick fence's info" +
				" string, but superfences does not implement that rule -- it renders a" +
				" fully YAML-lexed block. The scanner must not apply the CommonMark rule.",
			markdown: bt + "yaml title=\"`inventory.yaml`\"\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "attribute list with yaml first",
			why:      "{.yaml .annotate} is idiomatic Material and renders as YAML",
			markdown: bt + "{.yaml .annotate}\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "language followed by an attribute list",
			why:      "yaml{.annotate} is the other Material spelling of the same thing",
			markdown: bt + "yaml{.annotate}\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},
		{
			name: "attribute list with yaml second is NOT yaml",
			why: "superfences takes the FIRST class as the language, so {.annotate .yaml}" +
				" highlights as annotate, not yaml. Rejecting it matches the renderer.",
			markdown: bt + "{.annotate .yaml}\na: 1\n" + bt,
			want:     nil,
		},
		{
			name:     "language with a comma-separated option",
			why:      "yaml,linenums is admitted: the first token before the comma is the language",
			markdown: bt + "yaml,linenums\na: 1\n" + bt,
			want:     []string{"a: 1"},
		},

		// ---- non-YAML info strings ----
		{
			name:     "language with a yaml prefix is not yaml",
			why:      "yamlfoo is a different lexer; a prefix match would over-discover",
			markdown: bt + "yamlfoo\na: 1\n" + bt,
			want:     nil,
		},
		{
			name:     "hyphenated language containing yaml is not yaml",
			why:      "yaml-diff and not-yaml are their own lexers",
			markdown: bt + "yaml-diff\na: 1\n" + bt + "\n\n" + bt + "not-yaml\nb: 2\n" + bt,
			want:     nil,
		},
		{
			name:     "bare dot-prefixed language outside braces is not yaml",
			why:      ".yaml is only a class inside an attribute list, never a bare language",
			markdown: bt + ".yaml\na: 1\n" + bt,
			want:     nil,
		},
		{
			name:     "empty attribute list is not yaml",
			why:      "{} declares no class, so no language",
			markdown: bt + "{}\na: 1\n" + bt,
			want:     nil,
		},
		{
			name:     "no info string is not yaml",
			why:      "an unlabelled fence is plain text to the renderer",
			markdown: bt + "\na: 1\n" + bt,
			want:     nil,
		},

		// ---- tilde fences ----
		{
			name:     "tilde fence",
			why:      "~~~yaml renders as YAML on the pinned toolchain, so it must be discovered",
			markdown: "~~~yaml\na: 1\n~~~",
			want:     []string{"a: 1"},
		},
		{
			name:     "four-tilde fence",
			why:      "fences may be longer than three; length is not the signal",
			markdown: "~~~~yaml\na: 1\n~~~~",
			want:     []string{"a: 1"},
		},

		// ---- closer symmetry ----
		{
			name: "backticks do not close a tilde fence",
			why: "if the closer did not widen with the opener, one tilde fence would" +
				" swallow the rest of the page into a single unparseable block",
			markdown: "~~~yaml\na: 1\n" + bt + "\nb: 2\n~~~",
			want:     []string{"a: 1\n" + bt + "\nb: 2"},
		},
		{
			name:     "tildes do not close a backtick fence",
			why:      "the mirror of the case above",
			markdown: bt + "yaml\na: 1\n~~~\nb: 2\n" + bt,
			want:     []string{"a: 1\n~~~\nb: 2"},
		},
		{
			name:     "a shorter run does not close a longer fence",
			why:      "CommonMark requires the closer to be at least as long as the opener",
			markdown: "````yaml\na: 1\n" + bt + "\nb: 2\n````",
			want:     []string{"a: 1\n" + bt + "\nb: 2"},
		},

		// ---- nesting ----
		{
			name: "a yaml fence quoted inside a longer fence is not discovered",
			why: "mkdocs does not render the inner block as YAML either, so the scanner" +
				" matches the renderer; rescanning fence bodies would over-discover",
			markdown: "````text\n" + bt + "yaml\na: 1\n" + bt + "\n````",
			want:     nil,
		},

		// ---- indentation contexts ----
		{
			name:     "fence indented inside a list item",
			why:      "five of this repo's pages put examples inside numbered steps",
			markdown: "1. Step:\n\n   " + bt + "yaml\n   a: 1\n     b: 2\n   " + bt,
			want:     []string{"a: 1\n  b: 2"},
		},
		{
			name:     "fence inside a content tab",
			why:      "pymdownx.tabbed indents the fence under the tab label",
			markdown: "=== \"Tab\"\n\n    " + bt + "yaml\n    a: 1\n    " + bt,
			want:     []string{"a: 1"},
		},
		{
			name:     "fence inside an admonition",
			why:      "admonitions indent their body the same way",
			markdown: "!!! note\n\n    " + bt + "yaml\n    a: 1\n    " + bt,
			want:     []string{"a: 1"},
		},

		// ---- blockquotes ----
		{
			name: "fence inside a blockquote",
			why: "> ```yaml renders as a real highlighted YAML block and no markdownlint" +
				" rule forbids it, so leaving it undiscovered would be a silent hole",
			markdown: "> " + bt + "yaml\n> a: 1\n>   b: 2\n> " + bt,
			want:     []string{"a: 1\n  b: 2"},
		},
		{
			name: "blockquoted fence whose body contains a bare > blank line",
			why: "MUTATION GAP: every body line of the case above carries the full \"> \"" +
				" prefix, so plain TrimPrefix reproduces it and stripFencePrefix's" +
				" blockquote branch survived the whole table. A blank line inside a" +
				" blockquote is written \">\" with no trailing space -- it does NOT carry" +
				" the opener's prefix. Without the branch, \">\" leaks into the body, where" +
				" YAML reads it as a folded-scalar indicator and the block stops parsing as" +
				" what the page shows.",
			markdown: "> " + bt + "yaml\n> a: 1\n>\n> b: 2\n> " + bt,
			want:     []string{"a: 1\n\nb: 2"},
		},
		{
			name: "blockquoted fence indented two spaces after the marker",
			why: "the opener pattern used to consume at most ONE space per \">\", so a" +
				" second space made the anchored match fail outright and the block went" +
				" undiscovered -- the silent-hole shape. The pinned toolchain renders it" +
				" as language-yaml like any other.",
			markdown: ">  " + bt + "yaml\n>  a: 1\n>  " + bt,
			want:     []string{"a: 1"},
		},
		{
			name: "blockquoted list item indents its fence four spaces",
			why: "> 1. Step: puts its fence at \">    \" -- the natural shape, and the one" +
				" the one-space rule was furthest from matching. The list indent belongs to" +
				" the prefix, so it is stripped with it and relative indent survives.",
			markdown: "> 1. Step:\n>\n>    " + bt + "yaml\n>    a: 1\n>      b: 2\n>    " + bt,
			want:     []string{"a: 1\n  b: 2"},
		},
		{
			name: "blockquoted list item fence with a bare > blank line in its body",
			why: "INTERACTION: the opener pattern allows many spaces per \">\" while" +
				" blockquotePrefixPattern -- which backs stripFencePrefix's fallback and" +
				" isClosingFence -- still allows one. A bare \">\" body line takes exactly" +
				" that fallback, so this pins that the two patterns still agree on the" +
				" only line where they could disagree.",
			markdown: ">    " + bt + "yaml\n>    a: 1\n>\n>    b: 2\n>    " + bt,
			want:     []string{"a: 1\n\nb: 2"},
		},
		{
			name: "nested blockquote",
			why: "the gate comment claims blockquoted fences are discovered at any" +
				" nesting depth; pin the claim rather than assume it",
			markdown: "> > " + bt + "yaml\n> > a: 1\n> > " + bt,
			want:     []string{"a: 1"},
		},

		// ---- raw HTML ----
		{
			name: "fence inside raw HTML with no markdown attribute",
			why: "the gate comment used to declare this a known gap. It is not one:" +
				" superfences is a PREPROCESSOR, so it lexes the fence before the HTML" +
				" block matters and the page renders language-yaml with or without" +
				" markdown=\"1\". This scanner is line-based and agrees. Pinned because a" +
				" deleted disclaimer is worth less than a tested fact.",
			markdown: "<div>\n\n" + bt + "yaml\na: 1\n" + bt + "\n\n</div>",
			want:     []string{"a: 1"},
		},

		// ---- multiple blocks and desync ----
		{
			name:     "several blocks on one page are all found",
			why:      "the scanner must resume correctly after each fence",
			markdown: bt + "yaml\na: 1\n" + bt + "\n\ntext\n\n" + bt + "yaml\nb: 2\n" + bt,
			want:     []string{"a: 1", "b: 2"},
		},
		{
			name: "DESYNC REGRESSION: a skipped fence must not hide later blocks",
			why: "the P1-1 defect was not only that the odd fence was skipped -- the" +
				" scanner `continue`d instead of advancing past the closer, so it resumed" +
				" mid-body and lost every legitimate block after it. A naive fix to the" +
				" info-string handling reintroduces exactly this.",
			markdown: bt + "sh\necho hi\n" + bt + "\n\n" + bt + "not-yaml\nx: 1\n" + bt +
				"\n\n" + bt + "yaml\nreal: block\n" + bt,
			want: []string{"real: block"},
		},
		{
			name: "DESYNC REGRESSION: an odd info string must not hide later blocks",
			why: "the exact live payload: a backtick-bearing title followed by a real" +
				" block. Both were lost, and both gates stayed green.",
			markdown: bt + "text title=\"`x`\"\nnoise\n" + bt + "\n\n" + bt +
				"yaml\nreal: block\n" + bt,
			want: []string{"real: block"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blocks := extractFencedYAML("docs/scanner-test.md", tc.markdown)

			got := make([]string, 0, len(blocks))
			for _, block := range blocks {
				got = append(got, block.body)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("discovered %d YAML block(s), want %d.\nwhy: %s\ngot:  %q\nwant: %q",
					len(got), len(tc.want), tc.why, got, tc.want)
			}

			for i := range tc.want {
				if strings.TrimRight(got[i], "\n") != tc.want[i] {
					t.Errorf("block %d body mismatch.\nwhy: %s\ngot:  %q\nwant: %q",
						i, tc.why, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestInfoStringLanguage pins the info-string parser on its own, so a failure
// says which layer broke: this one, or the fence walker above it.
func TestInfoStringLanguage(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"yaml":                    "yaml",
		"yml":                     "yml",
		"YAML":                    "yaml",
		"  yaml  ":                "yaml",
		`yaml title="inv.yaml"`:   "yaml",
		"yaml title=\"`inv`\"":    "yaml",
		"yaml{.annotate}":         "yaml",
		"yaml,linenums":           "yaml",
		"{.yaml .annotate}":       "yaml",
		"{ .yaml #id }":           "yaml",
		"{.annotate .yaml}":       "annotate",
		"{#id}":                   "",
		"{}":                      "",
		"":                        "",
		"yamlfoo":                 "yamlfoo",
		"not-yaml":                "not-yaml",
		".yaml":                   ".yaml",
		"text title=\"`x`\"":      "text",
		"json":                    "json",
		"mermaid":                 "mermaid",
		"sh":                      "sh",
		"console linenums=\"1\"":  "console",
		"{.yaml title=\"inv\"}":   "yaml",
		"{.yaml.annotate}":        "yaml.annotate",
		"~~~ignored~~~":           "~~~ignored~~~",
		"yaml ":                   "yaml",
		"yaml\ttitle=\"inv\"":     "yaml",
		"YML":                     "yml",
		"{.YAML}":                 "yaml",
		"diff-yaml":               "diff-yaml",
		"yaml-diff":               "yaml-diff",
		"{.diff .yaml}":           "diff",
		"kollect":                 "kollect",
		"{.text .yaml .annotate}": "text",
	}

	for info, want := range cases {
		t.Run(info, func(t *testing.T) {
			t.Parallel()

			if got := infoStringLanguage(info); got != want {
				t.Errorf("infoStringLanguage(%q) = %q, want %q", info, got, want)
			}
		})
	}
}

// TestIsClosingFence pins the closer rule: same character, at least as many of
// them, nothing else on the line.
func TestIsClosingFence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		line   string
		marker string
		want   bool
	}{
		{"```", "```", true},
		{"   ```   ", "```", true},
		{"````", "```", true},
		{"``", "```", false},
		{"~~~", "```", false},
		{"```", "~~~", false},
		{"~~~", "~~~", true},
		{"~~~~", "~~~", true},
		{"~~~", "~~~~", false},
		{"``` yaml", "```", false},
		{"```x", "```", false},
		{"", "```", false},
		{"a```", "```", false},
	}

	for _, tc := range cases {
		if got := isClosingFence(tc.line, tc.marker); got != tc.want {
			t.Errorf("isClosingFence(%q, %q) = %v, want %v", tc.line, tc.marker, got, tc.want)
		}
	}
}
