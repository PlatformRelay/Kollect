// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsInitCanceled(t *testing.T) {
	t.Parallel()

	if IsInitCanceled(nil) {
		t.Fatal("nil must not be canceled")
	}
	if !IsInitCanceled(ErrCanceled) {
		t.Fatal("ErrCanceled must be canceled")
	}
	if !IsInitCanceled(fmt.Errorf("%w: declined", ErrCanceled)) {
		t.Fatal("wrapped ErrCanceled must be canceled")
	}
	if IsInitCanceled(errors.New("other")) {
		t.Fatal("unrelated error must not be canceled")
	}
}

func TestTruncateInit(t *testing.T) {
	t.Parallel()

	if got := truncateInit("short", 10); got != "short" {
		t.Fatalf("short: got %q", got)
	}
	exact := strings.Repeat("a", 5)
	if got := truncateInit(exact, 5); got != exact {
		t.Fatalf("exact: got %q", got)
	}
	long := strings.Repeat("b", 12)
	got := truncateInit(long, 8)
	want := "bbbbbbbb\n…(truncated)"
	if got != want {
		t.Fatalf("long: got %q want %q", got, want)
	}
}

func TestMapInitPromptErr(t *testing.T) {
	t.Parallel()

	if got := mapInitPromptErr(nil); got != nil {
		t.Fatalf("nil: got %v", got)
	}
	if got := mapInitPromptErr(ErrCanceled); !errors.Is(got, ErrCanceled) {
		t.Fatalf("ErrCanceled: got %v", got)
	}
	if got := mapInitPromptErr(errors.New("interrupt")); !errors.Is(got, ErrCanceled) {
		t.Fatalf("interrupt exact: got %v", got)
	}
	if got := mapInitPromptErr(errors.New("user Interrupt via Ctrl-C")); !errors.Is(got, ErrCanceled) {
		t.Fatalf("interrupt substring: got %v", got)
	}
	other := errors.New("disk full")
	if got := mapInitPromptErr(other); !errors.Is(got, other) {
		t.Fatalf("passthrough: got %v", got)
	}
}

func TestStripInitANSI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "hello", want: "hello"},
		{name: "color reset", in: "\x1b[0mplain", want: "plain"},
		{name: "fg then text", in: "\x1b[31mred\x1b[0m", want: "red"},
		{name: "uppercase terminator", in: "\x1b[1Aup", want: "up"},
		{name: "unterminated esc", in: "pre\x1b[31", want: "pre"},
		{name: "esc at end", in: "x\x1b", want: "x"},
		{name: "mixed", in: "a\x1b[32mb\x1b[0mc", want: "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stripInitANSI(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestRequireInitNonEmpty(t *testing.T) {
	t.Parallel()

	if err := requireInitNonEmpty("ok"); err != nil {
		t.Fatalf("non-empty: %v", err)
	}
	if err := requireInitNonEmpty(""); err == nil {
		t.Fatal("empty must fail")
	}
	if err := requireInitNonEmpty("   \t"); err == nil {
		t.Fatal("whitespace must fail")
	}
}

func TestValidateInitResourceName(t *testing.T) {
	t.Parallel()

	if err := validateInitResourceName("good-name1"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := validateInitResourceName(""); err == nil {
		t.Fatal("empty must fail")
	}
	if err := validateInitResourceName("Bad_Name"); err == nil {
		t.Fatal("uppercase/underscore must fail")
	}
}

func TestConfirmInitOverwrite_missingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")
	opts := InitOptions{
		Stderr:   &bytes.Buffer{},
		Prompter: NewScriptedPrompter(nil),
	}
	if err := confirmInitOverwrite(opts, path, []byte("new")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}

func TestConfirmInitOverwrite_accept(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "existing.yaml")
	if err := os.WriteFile(path, []byte("old-content-"+strings.Repeat("x", 20)), 0o600); err != nil {
		t.Fatal(err)
	}
	yes := true
	stderr := &bytes.Buffer{}
	opts := InitOptions{
		Stderr:   stderr,
		Color:    false,
		Prompter: NewScriptedPrompter([]PromptAnswer{{Confirm: &yes}}),
	}
	newContent := []byte("brand-new-" + strings.Repeat("y", 900))
	if err := confirmInitOverwrite(opts, path, newContent); err != nil {
		t.Fatalf("accept: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "File already exists") {
		t.Fatalf("stderr missing exists notice: %q", out)
	}
	if !strings.Contains(out, "…(truncated)") {
		t.Fatalf("stderr missing truncation marker for long new content: %q", out)
	}
}

func TestConfirmInitOverwrite_decline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "existing.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	no := false
	opts := InitOptions{
		Stderr:   &bytes.Buffer{},
		Prompter: NewScriptedPrompter([]PromptAnswer{{Confirm: &no}}),
	}
	err := confirmInitOverwrite(opts, path, []byte("new"))
	if !IsInitCanceled(err) {
		t.Fatalf("decline want canceled, got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error should name path: %v", err)
	}
}

func TestConfirmInitOverwrite_promptInterrupt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "existing.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := InitOptions{
		Stderr:   &bytes.Buffer{},
		Prompter: NewScriptedPrompter([]PromptAnswer{{Err: errors.New("interrupt")}}),
	}
	err := confirmInitOverwrite(opts, path, []byte("new"))
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("interrupt want ErrCanceled, got %v", err)
	}
}

func TestConfirmInitOverwrite_readError(t *testing.T) {
	t.Parallel()

	// ReadFile on a directory is not IsNotExist — exercises the hard-error path.
	dir := t.TempDir()
	opts := InitOptions{
		Stderr:   &bytes.Buffer{},
		Prompter: NewScriptedPrompter(nil),
	}
	err := confirmInitOverwrite(opts, dir, []byte("new"))
	if err == nil {
		t.Fatal("expected read error for directory path")
	}
	if !strings.Contains(err.Error(), "stat ") {
		t.Fatalf("want stat-wrapped error, got %v", err)
	}
}
