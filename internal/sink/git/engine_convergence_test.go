// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// Engine-convergence red (T04, GTE-1): backend construction must independently reject
// spec.git.engine=cli with an error naming go-git — defence in depth behind admission validation.
// Was red until T09 landed the convergence (the engine switch used to accept cli and map it
// onto the CLI engine, config.go applyGitSpec). Deliberately written
// engine-less (string literal "cli", no Config{Engine: ...}): T09 removed the GitEngine field
// and both GitEngineCLI constants, and this test survived that unchanged.
func TestConfigFromSpec_rejectsCLIEngineNamingGoGit(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.SinkTypeGit,
		Endpoint: "https://example.com/kollect.git",
		Git:      &kollectdevv1alpha1.GitSpec{Engine: "cli"},
	}

	_, err := ConfigFromSpec(spec, nil)
	if err == nil {
		t.Fatal("ConfigFromSpec(engine=cli) error = nil, want a rejection naming go-git as the engine")
	}
	if msg := err.Error(); !strings.Contains(msg, "go-git") {
		t.Fatalf("ConfigFromSpec(engine=cli) error = %q, want the rejection to name go-git as the engine", msg)
	}
}

// Guard (green-by-construction, GTE-1 scenario 2): engine=go-git and an omitted engine keep
// building a backend unchanged, before and after the convergence.
func TestConfigFromSpec_acceptsGoGitAndDefault(t *testing.T) {
	t.Parallel()

	gitSpec := func(engine string) *kollectdevv1alpha1.GitSpec {
		return &kollectdevv1alpha1.GitSpec{Engine: engine}
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.SinkTypeGit,
		Endpoint: "https://example.com/kollect.git",
		Git:      gitSpec("go-git"),
	}
	if _, err := ConfigFromSpec(spec, nil); err != nil {
		t.Fatalf("ConfigFromSpec(engine=go-git) error = %v, want accepted", err)
	}

	spec.Git = gitSpec("")
	if _, err := ConfigFromSpec(spec, nil); err != nil {
		t.Fatalf("ConfigFromSpec(engine omitted) error = %v, want accepted", err)
	}
}
