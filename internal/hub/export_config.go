// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"os"
	"strings"
	"time"
)

const (
	envHubExportNamespace = "KOLLECT_HUB_EXPORT_NAMESPACE"
	envHubSinkRefs        = "KOLLECT_HUB_SINK_REFS"
)

// ExportConfig configures hub-side parallel sink fan-out after spoke merge.
type ExportConfig struct {
	ExportNamespace   string
	SinkRefs          []string
	ExportMinInterval time.Duration
}

// ExportEnabled reports whether post-merge sink export is configured.
func (c ExportConfig) ExportEnabled() bool {
	return len(c.SinkRefs) > 0 && strings.TrimSpace(c.ExportNamespace) != ""
}

// ExportConfigFromEnv reads hub export sink refs and namespace for namespaced KollectSink resolution.
func ExportConfigFromEnv() ExportConfig {
	return ExportConfig{
		ExportNamespace: strings.TrimSpace(os.Getenv(envHubExportNamespace)),
		SinkRefs:        parseCSVEnv(os.Getenv(envHubSinkRefs)),
	}
}

func parseCSVEnv(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if _, ok := seen[part]; ok {
			continue
		}

		seen[part] = struct{}{}
		out = append(out, part)
	}

	return out
}
