// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package integrationtest

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// recordingTB captures Skipf/Fatalf instead of stopping the goroutine, so a test can assert which
// one SkipDockerUnavailable chose.
type recordingTB struct {
	testing.TB
	skipped string
	failed  string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Skipf(format string, args ...any) { r.skipped = fmt.Sprintf(format, args...) }

func (r *recordingTB) Fatalf(format string, args ...any) { r.failed = fmt.Sprintf(format, args...) }

func TestDockerRequired(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"false", false},
		{"0", false},
		{"true", true},
		{"1", true},
		// An unparseable value must not silently downgrade CI to skipping.
		{"yes-please", true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			t.Setenv(RequireDockerEnv, tc.value)

			if got := DockerRequired(); got != tc.want {
				t.Fatalf("DockerRequired() with %s=%q = %v, want %v", RequireDockerEnv, tc.value, got, tc.want)
			}
		})
	}
}

func TestSkipDockerUnavailable_skipsWhenNotRequired(t *testing.T) {
	t.Setenv(RequireDockerEnv, "")

	rec := &recordingTB{}
	SkipDockerUnavailable(rec, errors.New("cannot connect to the docker daemon"))

	if rec.failed != "" {
		t.Fatalf("failed = %q, want no failure when Docker is optional", rec.failed)
	}

	if !strings.Contains(rec.skipped, "cannot connect to the docker daemon") {
		t.Fatalf("skipped = %q, want a skip naming the cause", rec.skipped)
	}
}

func TestSkipDockerUnavailable_failsWhenRequired(t *testing.T) {
	t.Setenv(RequireDockerEnv, "true")

	rec := &recordingTB{}
	SkipDockerUnavailable(rec, errors.New("permission denied"))

	if rec.skipped != "" {
		t.Fatalf("skipped = %q, want no skip when Docker is required", rec.skipped)
	}

	if !strings.Contains(rec.failed, RequireDockerEnv) || !strings.Contains(rec.failed, "permission denied") {
		t.Fatalf("failed = %q, want a failure naming %s and the cause", rec.failed, RequireDockerEnv)
	}
}
