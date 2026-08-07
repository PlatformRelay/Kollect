// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// completionName strips the optional TAB-delimited description from a cobra.Completion.
func completionName(c cobra.Completion) string {
	name, _, _ := strings.Cut(c, "\t")
	return name
}

func TestFormatCompletions_coversParseableValues(t *testing.T) {
	t.Parallel()

	got, directive := cobra.FixedCompletions(formatCompletions, cobra.ShellCompDirectiveNoFileComp)(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}

	want := []string{"ndjson", "yaml", "json"}
	for i, w := range want {
		if i >= len(got) {
			t.Fatalf("got %d completions, want at least %d", len(got), len(want))
		}

		if name := completionName(got[i]); name != w {
			t.Errorf("completion[%d] = %q, want %q", i, name, w)
		}
	}
}

func TestLogLevelCompletions_matchesLogLevelsMap(t *testing.T) {
	t.Parallel()

	got := make(map[string]struct{}, len(logLevelCompletions))
	for _, c := range logLevelCompletions {
		got[completionName(c)] = struct{}{}
	}

	for name := range logLevels {
		if _, ok := got[name]; !ok {
			t.Errorf("logLevelCompletions missing %q from logLevels", name)
		}
	}

	if len(got) != len(logLevels) {
		t.Errorf("logLevelCompletions has %d entries, want %d", len(got), len(logLevels))
	}
}

func TestContextCompletionFunc_listsKubeconfigContexts(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("kubeconfig", path, "")

	got, directive := contextCompletionFunc(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}

	want := []string{"dev", "prod-eu-1", "prod-us-1", "staging-canary"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestContextCompletionFunc_filtersByPrefix(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("kubeconfig", path, "")

	got, _ := contextCompletionFunc(cmd, nil, "prod")
	want := []string{"prod-eu-1", "prod-us-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestContextCompletionFunc_completesLastCommaSegment(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("kubeconfig", path, "")

	got, _ := contextCompletionFunc(cmd, nil, "dev,prod")
	want := []string{"dev,prod-eu-1", "dev,prod-us-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestContextCompletionFunc_missingKubeconfigReturnsNoCompletions(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("kubeconfig", "/nonexistent/kubeconfig", "")

	got, directive := contextCompletionFunc(cmd, nil, "")
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestCollectCmd_contextFlagCompletionIsWired(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	cmd, _ := newCollectCmd()

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetArgs([]string{cobra.ShellCompRequestCmd, "--kubeconfig", path, "--context", "prod"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(__complete) error = %v", err)
	}

	if got := out.String(); !bytes.Contains([]byte(got), []byte("prod-eu-1")) {
		t.Errorf("completion output = %q, want it to contain %q", got, "prod-eu-1")
	}
}

func TestCollectCmd_formatFlagCompletionIsWired(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetArgs([]string{cobra.ShellCompRequestCmd, "--format", ""})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(__complete) error = %v", err)
	}

	if got := out.String(); !bytes.Contains([]byte(got), []byte("ndjson")) {
		t.Errorf("completion output = %q, want it to contain %q", got, "ndjson")
	}
}

func TestInitCmd_contextFlagCompletionIsWired(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	cmd, _ := newInitCmd()

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetArgs([]string{cobra.ShellCompRequestCmd, "--kubeconfig", path, "--context", "staging"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(__complete) error = %v", err)
	}

	if got := out.String(); !bytes.Contains([]byte(got), []byte("staging-canary")) {
		t.Errorf("completion output = %q, want it to contain %q", got, "staging-canary")
	}
}
