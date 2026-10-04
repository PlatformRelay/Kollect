// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package integrationtest

import (
	"os"
	"strconv"
	"testing"
)

// RequireDockerEnv names the environment variable that makes Docker a hard requirement. CI sets it
// on the integration job, where a missing daemon must fail the run instead of skipping every
// container-backed test and reporting green.
const RequireDockerEnv = "KOLLECT_REQUIRE_DOCKER"

// DockerRequired reports whether RequireDockerEnv demands Docker. Unset or a false boolean keeps
// the developer default of skipping; any other value, including an unparseable one, requires it.
func DockerRequired() bool {
	value, ok := os.LookupEnv(RequireDockerEnv)
	if !ok || value == "" {
		return false
	}

	required, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}

	return required
}

// SkipDockerUnavailable handles a container start error already classified as Docker being
// unavailable: it skips t, or fails it when DockerRequired.
func SkipDockerUnavailable(t testing.TB, err error) {
	t.Helper()

	if DockerRequired() {
		t.Fatalf("docker required (%s is set) but unavailable: %v", RequireDockerEnv, err)

		return
	}

	t.Skipf("docker not available: %v", err)
}
