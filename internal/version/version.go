// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package version reports build-time metadata for both binaries (VERSION-01: the manager
// built by cmd/main.go and the kollect-pipeline CLI built by cmd/cli/main.go). The defaults
// below apply to `go build`/`go run`/`go test` without -ldflags; each of the four release
// build sites (Taskfile.yml's build/build:cli tasks, Dockerfile, Dockerfile.pipeline) injects
// the real values via -X at build time.
package version

import "fmt"

var (
	// Version is the release tag this binary was built from (e.g. "v0.18.0"), or "dev".
	Version = "dev"
	// Commit is the short git SHA this binary was built from, or "unknown".
	Commit = "unknown"
	// Date is the UTC build timestamp in RFC3339, or "unknown".
	Date = "unknown"
)

// String renders the three fields as one human-readable line, e.g.
// "v0.18.0 (commit a1b2c3d, built 2026-08-07T12:00:00Z)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
