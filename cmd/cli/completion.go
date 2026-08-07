// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// formatCompletions lists the valid --format values (ADR-0802), default first.
var formatCompletions = []string{
	cobra.CompletionWithDesc("ndjson", "one compact JSON object per line (default)"),
	cobra.CompletionWithDesc("yaml", "a ---separated multi-document YAML stream"),
	cobra.CompletionWithDesc("json", "a single indented JSON array of all records"),
}

// logLevelCompletions lists the valid --log-level values, sorted for a stable order.
var logLevelCompletions = sortedLogLevelNames()

func sortedLogLevelNames() []string {
	names := make([]string, 0, len(logLevels))
	for name := range logLevels {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// registerFormatCompletion wires --format completion onto cmd.
func registerFormatCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("format",
		cobra.FixedCompletions(formatCompletions, cobra.ShellCompDirectiveNoFileComp))
}

// registerLogLevelCompletion wires --log-level completion onto cmd.
func registerLogLevelCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("log-level",
		cobra.FixedCompletions(logLevelCompletions, cobra.ShellCompDirectiveNoFileComp))
}

// registerContextCompletion wires dynamic --context completion onto cmd: it loads the same
// kubeconfig the run itself would use (effectiveKubeconfigPath, honoring a --kubeconfig already
// typed on the command line) and offers its context names, so a typo is caught by the shell
// before it becomes the fatal "not found in kubeconfig" error from resolveContexts.
func registerContextCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("context", contextCompletionFunc)
}

// contextCompletionFunc implements --context completion. It handles the collect subcommand's
// comma-separated form ("--context prod-a,prod-<TAB>") by completing only the segment after the
// last comma and re-prepending the already-typed prefix to each suggestion.
func contextCompletionFunc(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kubeconfigFlag, _ := cmd.Flags().GetString("kubeconfig")

	cfg, err := clientcmd.LoadFromFile(effectiveKubeconfigPath(kubeconfigFlag))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	leading, prefix := "", toComplete
	if idx := strings.LastIndex(toComplete, ","); idx >= 0 {
		leading, prefix = toComplete[:idx+1], toComplete[idx+1:]
	}

	names := make([]string, 0, len(cfg.Contexts))

	for name := range cfg.Contexts {
		if strings.HasPrefix(name, prefix) {
			names = append(names, leading+name)
		}
	}

	sort.Strings(names)

	return names, cobra.ShellCompDirectiveNoFileComp
}
