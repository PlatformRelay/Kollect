// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmd_validateSubcommandRegistered(t *testing.T) {
	t.Parallel()

	root, _ := buildRoot()

	found := false

	for _, c := range root.Commands() {
		if c.Name() == "validate" {
			found = true

			break
		}
	}

	if !found {
		t.Fatal("expected validate subcommand to be registered on kollect-pipeline root")
	}
}

func TestValidateCmd_helpExitsZero(t *testing.T) {
	t.Parallel()

	cmd, _ := newValidateCmd()
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--help) error = %v", err)
	}
}

func TestValidateCmd_missingConfigFlagReturnsError(t *testing.T) {
	t.Parallel()

	cmd, exitCode := newValidateCmd()
	cmd.SetArgs([]string{})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--config is required") {
		t.Fatalf("Execute() error = %v, want --config is required", err)
	}

	if *exitCode != ExitFatalError {
		t.Errorf("exitCode = %d, want ExitFatalError", *exitCode)
	}
}

func TestValidateCmd_missingConfigDirReturnsError(t *testing.T) {
	t.Parallel()

	cmd, exitCode := newValidateCmd()
	cmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "does-not-exist")})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for a nonexistent --config dir, got nil")
	}

	if *exitCode != ExitFatalError {
		t.Errorf("exitCode = %d, want ExitFatalError", *exitCode)
	}
}

func TestValidateCmd_validConfigSucceeds(t *testing.T) {
	t.Parallel()

	cmd, exitCode := newValidateCmd()

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", writeValidConfigDir(t)})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if *exitCode != ExitSuccess {
		t.Errorf("exitCode = %d, want ExitSuccess", *exitCode)
	}

	if !strings.Contains(out.String(), "valid") {
		t.Errorf("stdout = %q, want it to report validity", out.String())
	}
}

func TestValidateCmd_validConfigWithSinkSucceeds(t *testing.T) {
	t.Parallel()

	dir := writeValidConfigDir(t)
	sinkYAML := `apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: s1
  namespace: default
spec:
  type: git
  endpoint: https://gitlab.example.com/platform-team/cluster-inventory.git
  pathTemplate: "clusters/{cluster}/{namespace}/{name}.yaml"
  cluster: my-cluster
  git:
    branch: main
    pushPolicy: Commit
    auth:
      type: token
  secretRef:
    name: git-credentials
`
	if err := os.WriteFile(filepath.Join(dir, "sink.yaml"), []byte(sinkYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, exitCode := newValidateCmd()

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if *exitCode != ExitSuccess {
		t.Errorf("exitCode = %d, want ExitSuccess", *exitCode)
	}
}

func TestValidateCmd_invalidProfileFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Missing spec.targetGVK.kind (required).
	profile := `apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: p1
  namespace: default
spec:
  targetGVK:
    version: v1
`
	if err := os.WriteFile(filepath.Join(dir, "profile.yaml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, exitCode := newValidateCmd()

	var errOut bytes.Buffer

	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--config", dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for an invalid profile, got nil")
	}

	if *exitCode != ExitFatalError {
		t.Errorf("exitCode = %d, want ExitFatalError", *exitCode)
	}

	if !strings.Contains(errOut.String(), `KollectProfile "p1" is invalid`) {
		t.Errorf("stderr = %q, want it to name the invalid KollectProfile", errOut.String())
	}
}

func TestValidateCmd_invalidTargetFails(t *testing.T) {
	t.Parallel()

	dir := writeValidConfigDir(t) // profile "p1"
	// Duplicate includedNamespaces entry is rejected by ValidateCollectionFilterSpec.
	target := `apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: t2
  namespace: default
spec:
  profileRef: p1
  includedNamespaces: [a, a]
`
	if err := os.WriteFile(filepath.Join(dir, "target2.yaml"), []byte(target), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, exitCode := newValidateCmd()

	var errOut bytes.Buffer

	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--config", dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for an invalid target, got nil")
	}

	if *exitCode != ExitFatalError {
		t.Errorf("exitCode = %d, want ExitFatalError", *exitCode)
	}

	if !strings.Contains(errOut.String(), `KollectTarget "t2" is invalid`) {
		t.Errorf("stderr = %q, want it to name the invalid KollectTarget", errOut.String())
	}
}

func TestValidateCmd_invalidSinkFails(t *testing.T) {
	t.Parallel()

	dir := writeValidConfigDir(t)
	sinkYAML := `apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: bad-sink
  namespace: default
spec:
  type: bogus
`
	if err := os.WriteFile(filepath.Join(dir, "sink.yaml"), []byte(sinkYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, exitCode := newValidateCmd()

	var errOut bytes.Buffer

	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--config", dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for an invalid sink type, got nil")
	}

	if *exitCode != ExitFatalError {
		t.Errorf("exitCode = %d, want ExitFatalError", *exitCode)
	}

	if !strings.Contains(errOut.String(), `KollectSnapshotSink "bad-sink" is invalid`) {
		t.Errorf("stderr = %q, want it to name the invalid KollectSnapshotSink", errOut.String())
	}
}

func TestValidateCmd_profileWarningGoesToStderrNotFatal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A JSONPath filter expression is valid-but-discouraged (ProfileWarnings), not an error.
	profile := `apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: p1
  namespace: default
spec:
  targetGVK:
    version: v1
    kind: Pod
  attributes:
    - name: readyContainers
      path: "$.status.containerStatuses[?(@.ready==true)].name"
`
	if err := os.WriteFile(filepath.Join(dir, "profile.yaml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd, exitCode := newValidateCmd()

	var out, errOut bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--config", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v (a warning must not fail validate)", err)
	}

	if *exitCode != ExitSuccess {
		t.Errorf("exitCode = %d, want ExitSuccess", *exitCode)
	}

	if !strings.Contains(errOut.String(), "warning:") {
		t.Errorf("stderr = %q, want a JSONPath-filter warning", errOut.String())
	}
}
