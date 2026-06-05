// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// CacheOptionsForWatchNamespaces returns controller-runtime cache options that restrict the
// manager informer cache to the given namespaces. An empty or nil slice watches all namespaces.
func CacheOptionsForWatchNamespaces(namespaces []string) cache.Options {
	trimmed := trimNamespaces(namespaces)
	if len(trimmed) == 0 {
		return cache.Options{}
	}

	defaultNamespaces := make(map[string]cache.Config, len(trimmed))
	for _, ns := range trimmed {
		defaultNamespaces[ns] = cache.Config{}
	}

	return cache.Options{DefaultNamespaces: defaultNamespaces}
}

// ParseWatchNamespaces splits a comma-separated namespace list from a CLI flag.
func ParseWatchNamespaces(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	return trimNamespaces(parts)
}

func trimNamespaces(namespaces []string) []string {
	if len(namespaces) == 0 {
		return nil
	}

	trimmed := make([]string, 0, len(namespaces))
	seen := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		if _, ok := seen[ns]; ok {
			continue
		}
		seen[ns] = struct{}{}
		trimmed = append(trimmed, ns)
	}

	return trimmed
}
