// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_validDir(t *testing.T) {
	t.Parallel()

	result, err := LoadConfig("testdata/valid")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(result.Profiles))
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}
	if len(result.Sinks) != 1 {
		t.Fatalf("expected 1 sink, got %d", len(result.Sinks))
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", result.Warnings)
	}
	if result.Profiles[0].Spec.TargetGVK.Kind != "Secret" {
		t.Errorf("expected profile targetGVK.kind = Secret, got %q", result.Profiles[0].Spec.TargetGVK.Kind)
	}
}

func TestLoadConfig_multiDocYAML(t *testing.T) {
	t.Parallel()

	result, err := LoadConfig("testdata/multidoc")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Profiles) != 1 || len(result.Targets) != 1 {
		t.Fatalf("expected 1 profile + 1 target from multidoc file, got %d profiles, %d targets",
			len(result.Profiles), len(result.Targets))
	}
}

func TestLoadConfig_unknownKindIsSkipped(t *testing.T) {
	t.Parallel()

	result, err := LoadConfig("testdata/unknown_kind")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected at least one warning for unknown kind")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "SomethingUnknown") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning mentioning the unknown kind, got %v", result.Warnings)
	}
}

func TestLoadConfig_missingProfileRef(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("testdata/missing_profile")
	if err == nil {
		t.Fatal("expected error for missing profileRef, got nil")
	}
	if !strings.Contains(err.Error(), "orphan-target") || !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("expected error to mention target name and profileRef, got: %v", err)
	}
}

func TestLoadConfig_malformedYAML(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("testdata/malformed")
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestLoadConfig_emptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Profiles) != 0 || len(result.Targets) != 0 || len(result.Sinks) != 0 || len(result.Secrets) != 0 {
		t.Errorf("expected all-empty result, got %+v", result)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", result.Warnings)
	}
}

func TestLoadConfig_dirDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("/nonexistent/path/for/kollect-pipeline-tests")
	if err == nil {
		t.Fatal("expected error for nonexistent dir, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got: %v", err)
	}
}

func TestLoadConfig_nonYAMLFilesAreSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/README.md", []byte("# not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected non-YAML files to be silently skipped, got warnings: %v", result.Warnings)
	}
}

// secretFixtureYAML is a Secret manifest with a placeholder base64 value, not a real credential.
//
//nolint:gosec // G101: test fixture, not a credential
const secretFixtureYAML = `apiVersion: v1
kind: Secret
metadata:
  name: sink-creds
  namespace: default
data:
  greeting: aGVsbG8=
`

func TestLoadConfig_loadsSecrets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/secret.yaml", []byte(secretFixtureYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(result.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(result.Secrets))
	}
	if result.Secrets[0].Name != "sink-creds" {
		t.Errorf("expected secret name sink-creds, got %q", result.Secrets[0].Name)
	}
}
