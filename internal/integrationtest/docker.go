// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package integrationtest holds shared helpers for Docker-gated integration tests.
package integrationtest

import "strings"

// IsDockerUnavailable reports whether err indicates the Docker daemon is missing or unreachable.
func IsDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "docker provider") ||
		strings.Contains(msg, "rootless docker")
}
