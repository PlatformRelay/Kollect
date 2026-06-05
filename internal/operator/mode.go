// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import (
	"os"
	"strings"
)

const (
	// ModeCluster is the default single-cluster operator (collect + export).
	ModeCluster = "cluster"
	// ModeHub runs hub ingest and spoke-report consumer (ADR-0501 L2).
	ModeHub = "hub"
	// ModeSpoke is a spoke cluster publishing deltas to a hub.
	ModeSpoke = "spoke"

	envMode = "KOLLECT_MODE"
)

// ResolveMode returns the effective operator mode from flag and KOLLECT_MODE env.
// Flag wins when non-empty; hubConsumer forces hub for backward compatibility.
func ResolveMode(flagValue string, hubConsumer bool) string {
	if hubConsumer {
		return ModeHub
	}

	if v := strings.TrimSpace(flagValue); v != "" {
		return normalizeMode(v)
	}

	if v := strings.TrimSpace(os.Getenv(envMode)); v != "" {
		return normalizeMode(v)
	}

	return ModeCluster
}

func normalizeMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ModeHub, "hub-consumer":
		return ModeHub
	case ModeSpoke:
		return ModeSpoke
	case ModeCluster, "single", "":
		return ModeCluster
	default:
		return ModeCluster
	}
}

// IsHubMode reports whether the process should run hub consumer wiring.
func IsHubMode(mode string) bool {
	return normalizeMode(mode) == ModeHub
}
