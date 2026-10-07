// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// Engine-convergence red (T04, GTE-1): admission validation must reject spec.git.engine=cli
// with an error naming go-git as the engine. Red until T09 lands the convergence: today the
// switch accepts both values (git.go), so the test fails on the missing rejection.
func TestValidateGitSpec_rejectsCLINamingGoGit(t *testing.T) {
	t.Parallel()

	errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{Engine: "cli"},
	})
	if len(errs) != 1 {
		t.Fatalf("validateGitSpec(engine=cli) = %d errors, want 1 error naming go-git: %v", len(errs), errs)
	}
	if msg := errs[0].Error(); !strings.Contains(msg, "go-git") {
		t.Fatalf("validateGitSpec(engine=cli) error = %q, want the rejection to name go-git as the engine", msg)
	}
}

// Guard (green-by-construction, GTE-1 scenario 2): engine=go-git and an omitted engine keep
// passing admission unchanged, before and after the convergence.
func TestValidateGitSpec_acceptsGoGitAndDefault(t *testing.T) {
	t.Parallel()

	if errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{Engine: "go-git"},
	}); len(errs) != 0 {
		t.Fatalf("validateGitSpec(engine=go-git) = %v, want accepted", errs)
	}

	if errs := validateGitSpec(&kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypeGit,
		Git:  &kollectdevv1alpha1.GitSpec{},
	}); len(errs) != 0 {
		t.Fatalf("validateGitSpec(engine omitted) = %v, want accepted", errs)
	}
}
