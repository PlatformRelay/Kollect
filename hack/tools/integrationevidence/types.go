// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import "io"

// integrationTag is the build tag that marks a test file as an integration test.
const integrationTag = "integration"

// requireDockerEnv mirrors internal/integrationtest.RequireDockerEnv; this tool cannot import
// internal packages of the module it is checking without coupling to them.
const requireDockerEnv = "KOLLECT_REQUIRE_DOCKER"

type runMode string

const (
	modeRequired runMode = "required"
	modeExplore  runMode = "explore"
)

// testID names one test of one package, as go test -json reports it.
type testID struct {
	Package string
	Test    string
}

func (id testID) String() string { return id.Package + "." + id.Test }

// listedPackage is the subset of `go list -json` output discovery needs.
type listedPackage struct {
	ImportPath   string
	Dir          string
	TestGoFiles  []string
	XTestGoFiles []string
}

// analysisInput is everything analyze needs; it never reads the environment.
type analysisInput struct {
	// Expected counts the declarations of each expected test; a name can be declared in both the
	// internal and the external test package, and go test runs and reports each one.
	Expected map[testID]int
	// Outside names integration tests in packages the run did not select.
	Outside  []string
	Packages []string
	Stream   io.Reader
	ExitCode int
	Mode     runMode
}
