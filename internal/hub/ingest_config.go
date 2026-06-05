// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"os"
	"strconv"
	"strings"
)

const (
	envHubIngestPort        = "KOLLECT_HUB_INGEST_PORT"
	envHubIngestAuthMode    = "KOLLECT_HUB_INGEST_AUTH_MODE"
	envHubPlatformNamespace = "KOLLECT_PLATFORM_NAMESPACE"
	defaultIngestPort       = 8083
)

// PlatformNamespaceFromEnv returns the hub platform namespace for namespaced SAR checks.
func PlatformNamespaceFromEnv() string {
	return strings.TrimSpace(os.Getenv(envHubPlatformNamespace))
}

// IngestConfigFromEnv reads hub HTTP ingest settings for hub-consumer mode.
func IngestConfigFromEnv() (int32, string) {
	port := int32(defaultIngestPort)
	if raw := os.Getenv(envHubIngestPort); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p > 0 && p <= 65535 {
			port = int32(p) //nolint:gosec // G109: bounded to valid TCP port range
		}
	}

	mode := os.Getenv(envHubIngestAuthMode)
	if mode == "" {
		mode = IngestAuthModeKubernetes
	}

	return port, mode
}
