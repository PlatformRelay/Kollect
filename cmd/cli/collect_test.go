// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/pipeline"
	"github.com/platformrelay/kollect/internal/sink"
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

func TestCollectCmd_stdoutAndDryRunMutuallyExclusive(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", writeValidConfigDir(t), "--output", "-", "--dry-run"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --output - and --dry-run are mutually exclusive, got nil")
	}
}

func TestCollectCmd_formatWithNonStdoutOutputIsError(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", writeValidConfigDir(t), "--output", t.TempDir(), "--format", "yaml"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --format applies only with --output -, got nil")
	}
}

func TestCollectCmd_invalidFormatIsError(t *testing.T) {
	t.Parallel()

	cmd, _ := newCollectCmd()
	cmd.SetArgs([]string{"--config", writeValidConfigDir(t), "--output", "-", "--format", "toml"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: invalid --format toml, got nil")
	}
}

func TestCollectCmd_stdoutAndSinkYAMLAreAmbiguous(t *testing.T) {
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
	cmd.SetArgs([]string{"--config", dir, "--output", "-", "--kubeconfig", writeFixtureKubeconfig(t)})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --output - and Sink YAML are ambiguous, got nil")
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

// TestMapContextResultsToExit_extractionFailureWithExportIsPartial covers REL-02: a
// ContextResult that folded collect.ExtractionFailure into Errs (via buildContextResult)
// must exit 1 when something was still exported — never 0.
func TestMapContextResultsToExit_extractionFailureWithExportIsPartial(t *testing.T) {
	t.Parallel()

	got := mapContextResultsToExit([]pipeline.ContextResult{
		{Context: "a", Exported: 1, Errs: []error{errFixture{}}},
	})
	if got != ExitPartialFailure {
		t.Errorf("got %d, want ExitPartialFailure", got)
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "fixture error" }

// --- stdout emission wiring (REQ-PIPE-02) ---
//
// These drive the real `collect --output -` command through cobra with runAllContexts faked, so the
// stdout branch in runCollectPipeline (flatten records -> WriteStdoutRecords -> exit mapping) and the
// data-on-stdout/errors-on-stderr contract are exercised without a cluster.

// fakeRunAllContexts swaps the collection seam for the duration of a test.
func fakeRunAllContexts(t *testing.T, results ...pipeline.ContextResult) {
	t.Helper()

	orig := runAllContexts
	t.Cleanup(func() { runAllContexts = orig })
	runAllContexts = func(
		_ context.Context, _ []string, _ string, _ pipeline.LoadResult,
		_ kollectdevv1alpha1.KollectSinkSpec, _ map[string][]byte, _ *sink.Registry, _ []string, _ bool,
	) []pipeline.ContextResult {
		return results
	}
}

// stdoutRecord builds a minimal but valid export record for a target.
func stdoutRecord(ctxName, ns, name string) pipeline.StdoutRecord {
	return pipeline.StdoutRecord{
		Context:         ctxName,
		TargetNamespace: ns,
		TargetName:      name,
		Path:            "inventory/" + ns + "/" + name + ".yaml",
		Envelope: collect.ExportEnvelope{
			SchemaVersion: collect.ExportSchemaVersion,
			ItemCount:     1,
			Items:         []collect.Item{{Namespace: ns, Name: name, Version: "v1", Kind: "ConfigMap", UID: "u-" + name}},
		},
	}
}

func stdoutCollectArgs(t *testing.T) []string {
	t.Helper()

	return []string{"--config", writeValidConfigDir(t), "--output", "-", "--kubeconfig", writeFixtureKubeconfig(t)}
}

func TestCollectCmd_stdoutEmitsNDJSONToStdoutNotStderr(t *testing.T) {
	fakeRunAllContexts(t, pipeline.ContextResult{
		Context: "c1", Exported: 1, Records: []pipeline.StdoutRecord{stdoutRecord("c1", "default", "t1")},
	})

	cmd, code := newCollectCmd()

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(stdoutCollectArgs(t))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	line := strings.TrimSpace(out.String())
	if strings.Count(line, "\n") != 0 {
		t.Fatalf("expected exactly one NDJSON line, got:\n%s", out.String())
	}

	var got pipeline.StdoutRecord
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("stdout is not NDJSON (%q): %v", out.String(), err)
	}

	if got.TargetName != "t1" || got.Envelope.ItemCount != 1 {
		t.Errorf("record = %+v, want target t1 / itemCount 1", got)
	}

	if errBuf.Len() != 0 {
		t.Errorf("stderr should be empty on a clean run, got %q", errBuf.String())
	}

	if *code != ExitSuccess {
		t.Errorf("exit = %d, want ExitSuccess", *code)
	}
}

func TestCollectCmd_stdoutMultiContextRecordsInOrderErrorsToStderr(t *testing.T) {
	fakeRunAllContexts(t,
		pipeline.ContextResult{Context: "a", Exported: 1, Records: []pipeline.StdoutRecord{stdoutRecord("a", "ns", "t-a")}},
		pipeline.ContextResult{
			Context: "b", Exported: 1,
			Records: []pipeline.StdoutRecord{stdoutRecord("b", "ns", "t-b")},
			Errs:    []error{errFixture{}},
		},
	)

	cmd, code := newCollectCmd()

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(stdoutCollectArgs(t))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), out.String())
	}

	var first pipeline.StdoutRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 not NDJSON: %v", err)
	}

	if first.TargetName != "t-a" {
		t.Errorf("context order not preserved: first record target = %q, want t-a", first.TargetName)
	}

	// The context 'b' error must land on stderr, never mixed into the stdout records.
	if !strings.Contains(errBuf.String(), "fixture error") {
		t.Errorf("stderr missing the context error; got %q", errBuf.String())
	}

	if strings.Contains(out.String(), "fixture error") {
		t.Error("stdout was contaminated with an error message")
	}

	if *code != ExitPartialFailure {
		t.Errorf("exit = %d, want ExitPartialFailure (one context had errors)", *code)
	}
}

func TestCollectCmd_stdoutFormatJSONEmitsArray(t *testing.T) {
	fakeRunAllContexts(t, pipeline.ContextResult{
		Context: "c1", Exported: 1, Records: []pipeline.StdoutRecord{stdoutRecord("c1", "default", "t1")},
	})

	cmd, _ := newCollectCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append(stdoutCollectArgs(t), "--format", "json"))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var arr []pipeline.StdoutRecord
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("stdout is not a JSON array (%q): %v", out.String(), err)
	}

	if len(arr) != 1 || arr[0].TargetName != "t1" {
		t.Errorf("json array = %+v, want one record for t1", arr)
	}
}

// failingWriter errors on every Write, to exercise the fatal-output path.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk full") }

func TestCollectCmd_stdoutWriteFailureIsExitFatal(t *testing.T) {
	fakeRunAllContexts(t, pipeline.ContextResult{
		Context: "c1", Exported: 1, Records: []pipeline.StdoutRecord{stdoutRecord("c1", "default", "t1")},
	})

	cmd, code := newCollectCmd()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(stdoutCollectArgs(t))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a fatal write error, got nil")
	}

	if *code != ExitFatalError {
		t.Errorf("exit = %d, want ExitFatalError on stdout write failure", *code)
	}
}
