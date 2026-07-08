// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konih/kollect/internal/pipeline"
)

func TestCollectCmd_helpExitsZero(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--help) error = %v", err)
	}
}

func TestCollectCmd_missingConfigFlagReturnsError(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --config, got nil")
	}
}

func TestCollectCmd_invalidLogLevelReturnsExitFatal(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", t.TempDir(), "--log-level", "banana"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --log-level, got nil")
	}
}

// writeValidConfigDir writes a minimal but complete KollectProfile + KollectTarget pair
// (no Sink) so runCollectPipeline gets past config loading and context resolution, letting
// these tests exercise sink-resolution validation without needing a reachable cluster.
func writeValidConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	profile := `apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: p1
  namespace: default
spec:
  targetGVK:
    version: v1
    kind: Secret
`
	target := `apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: t1
  namespace: default
spec:
  profileRef: p1
`
	if err := os.WriteFile(filepath.Join(dir, "profile.yaml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target.yaml"), []byte(target), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestCollectCmd_zeroSinksNoOutputIsError(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", writeValidConfigDir(t), "--kubeconfig", writeFixtureKubeconfig(t)})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: no Sink YAML and no --output, got nil")
	}
}

func TestCollectCmd_outputAndSinkYAMLAreAmbiguous(t *testing.T) {
	t.Parallel()

	dir := writeValidConfigDir(t)
	sinkYAML := `apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: s1
  namespace: default
spec:
  type: local
  pathTemplate: "{namespace}/{name}.yaml"
`
	if err := os.WriteFile(filepath.Join(dir, "sink.yaml"), []byte(sinkYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", dir, "--output", t.TempDir(), "--kubeconfig", writeFixtureKubeconfig(t)})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --output and Sink YAML are ambiguous, got nil")
	}
}

func TestMapContextResultsToExit_allSucceeded(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{{Context: "a", Exported: 3}})
	if got != ExitSuccess {
		t.Errorf("got %d, want ExitSuccess", got)
	}
}

func TestMapContextResultsToExit_oneContextPartialFailureIsExit1(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Exported: 3, Errs: []error{errFixture{}}},
	})
	if got != ExitPartialFailure {
		t.Errorf("got %d, want ExitPartialFailure", got)
	}
}

func TestMapContextResultsToExit_oneContextFatalAmongSuccessesIsExit1(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Exported: 3},
		{Context: "b", Fatal: errFixture{}},
	})
	if got != ExitFatalError {
		t.Errorf("got %d, want ExitFatalError (worst-of across contexts)", got)
	}
}

func TestMapContextResultsToExit_allContextsFatalIsExit2(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Fatal: errFixture{}},
		{Context: "b", Fatal: errFixture{}},
	})
	if got != ExitFatalError {
		t.Errorf("got %d, want ExitFatalError", got)
	}
}

func TestMapContextResultsToExit_emptyResultsIsSuccess(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit(nil)
	if got != ExitSuccess {
		t.Errorf("got %d, want ExitSuccess", got)
	}
}

// TestMapContextResultsToExit_allTargetsSkippedNoErrsIsFatal guards against silently
// reporting success when every target was forbidden/transient/gvk-not-found: skipped
// targets produce no collect.RunResult.Errors entry (only SkippedTargets), so without this
// case a run where an RBAC-forbidden target skips everything would still exit 0.
func TestMapContextResultsToExit_allTargetsSkippedNoErrsIsFatal(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Exported: 0, Skipped: 1},
	})
	if got != ExitFatalError {
		t.Errorf("got %d, want ExitFatalError (nothing succeeded)", got)
	}
}

func TestMapContextResultsToExit_someTargetsSkippedWithExportsIsPartial(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Exported: 2, Skipped: 1},
	})
	if got != ExitPartialFailure {
		t.Errorf("got %d, want ExitPartialFailure", got)
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "fixture error" }
