// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

// resolveContexts loads kubeconfigPath and resolves patterns (the --context flag values)
// against its context names (ADR-0801 "Multi-context selection"):
//
//   - No patterns: only the kubeconfig's current-context.
//   - A literal name matching nothing: fatal (almost always a typo).
//   - A glob (containing * or ?) matching nothing: a warning, not fatal.
//   - The final de-duplicated, sorted union of all matches. Empty after all patterns
//     are applied: fatal (nothing to collect).
func resolveContexts(kubeconfigPath string, patterns []string) (contexts []string, warnings []string, err error) {
	cfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load kubeconfig %q: %w", kubeconfigPath, err)
	}

	if len(patterns) == 0 {
		if cfg.CurrentContext == "" {
			return nil, nil, fmt.Errorf("kubeconfig %q has no current-context and no --context was given", kubeconfigPath)
		}

		return []string{cfg.CurrentContext}, nil, nil
	}

	matched := make(map[string]struct{})

	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, "*?") {
			found := false

			for name := range cfg.Contexts {
				ok, matchErr := path.Match(pattern, name)
				if matchErr != nil {
					return nil, nil, fmt.Errorf("invalid --context pattern %q: %w", pattern, matchErr)
				}

				if ok {
					matched[name] = struct{}{}
					found = true
				}
			}

			if !found {
				warnings = append(warnings, fmt.Sprintf("--context %q matched no contexts in kubeconfig", pattern))
			}

			continue
		}

		if _, ok := cfg.Contexts[pattern]; !ok {
			return nil, nil, fmt.Errorf("--context %q not found in kubeconfig %q (typo?)", pattern, kubeconfigPath)
		}

		matched[pattern] = struct{}{}
	}

	if len(matched) == 0 {
		return nil, nil, fmt.Errorf("no contexts matched the given --context patterns %v", patterns)
	}

	contexts = make([]string, 0, len(matched))
	for name := range matched {
		contexts = append(contexts, name)
	}

	sort.Strings(contexts)

	return contexts, warnings, nil
}
