// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package integrationtest

import (
	"errors"
	"testing"
)

func TestIsDockerUnavailable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock"), true},
		// testcontainers-go on a host with no Docker at all.
		{errors.New("get provider: rootless Docker not found, failed to create Docker provider"), true},
		{errors.New("pull access denied for postgres"), false},
		{errors.New("start postgres: container exited with code 1"), false},
	}

	for _, tc := range cases {
		if got := IsDockerUnavailable(tc.err); got != tc.want {
			t.Errorf("IsDockerUnavailable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
