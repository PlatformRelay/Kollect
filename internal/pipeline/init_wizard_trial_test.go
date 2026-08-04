// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatInitTrialScreen_emitsExactCollectCommandsAndMarkedKubectl(t *testing.T) {
	t.Parallel()

	outDir := "/tmp/collect-config-example"
	got := FormatInitTrialScreen(outDir)

	wantStdout := `kollect-pipeline collect --config "/tmp/collect-config-example" --output -`
	wantLocal := `kollect-pipeline collect --config "/tmp/collect-config-example" --output ./inventory`
	wantApply := `kubectl apply -f "/tmp/collect-config-example"`

	if !strings.Contains(got, wantStdout) {
		t.Fatalf("trial screen missing exact stdout-preview command %q\ngot:\n%s", wantStdout, got)
	}
	if !strings.Contains(got, wantLocal) {
		t.Fatalf("trial screen missing exact local-dir trial command %q\ngot:\n%s", wantLocal, got)
	}
	if !strings.Contains(got, wantApply) {
		t.Fatalf("trial screen missing copyable kubectl apply %q\ngot:\n%s", wantApply, got)
	}
	if !strings.Contains(strings.ToLower(got), "future") ||
		!strings.Contains(strings.ToLower(got), "guidance") {
		t.Fatalf("kubectl apply must be clearly marked as future guidance only, got:\n%s", got)
	}
	if !strings.Contains(got, "CRD") && !strings.Contains(strings.ToLower(got), "authorization") {
		t.Fatalf("kubectl apply guidance must mention CRDs or authorization, got:\n%s", got)
	}
}

func TestRunInit_completionScreenEmitsTrialCommands(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtr(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeAll},
		{Select: InitFilterNone},
		{MultiSelect: []string{"name"}},
		{Input: "deployment-images"},
		{Confirm: boolPtr(true)},
	})
	var stderr bytes.Buffer
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources: []InitResourceInfo{deploymentResource()},
		},
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	out := stderr.String()
	wantStdout := FormatInitTrialStdoutCmd(outDir)
	wantLocal := FormatInitTrialLocalCmd(outDir)
	if !strings.Contains(out, wantStdout) {
		t.Fatalf("completion stderr missing stdout trial %q\ngot:\n%s", wantStdout, out)
	}
	if !strings.Contains(out, wantLocal) {
		t.Fatalf("completion stderr missing local trial %q\ngot:\n%s", wantLocal, out)
	}
	if !strings.Contains(out, FormatInitTrialKubectlApplyCmd(outDir)) {
		t.Fatalf("completion stderr missing kubectl apply guidance\ngot:\n%s", out)
	}
}

func TestValidateInitConfig_acceptsGeneratedProfileAndTarget(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	draft := initDraft{
		Name:     "deployment-images",
		Resource: deploymentResource(),
		Attributes: []initAttributeOpt{
			{Name: "name", Path: "$.metadata.name", Type: "string"},
		},
	}
	profileYAML, targetYAML, err := draft.RenderYAML()
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, initProfileFileName), profileYAML, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, initTargetFileName), targetYAML, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	if validateErr := ValidateInitConfig(outDir); validateErr != nil {
		t.Fatalf("ValidateInitConfig on generated YAML: %v", validateErr)
	}
}

func TestValidateInitConfig_invalidFieldFailsWithActionableError(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	draft := initDraft{
		Name:     "deployment-images",
		Resource: deploymentResource(),
		Attributes: []initAttributeOpt{
			{Name: "name", Path: "$.metadata.name", Type: "string"},
		},
	}
	profileYAML, targetYAML, err := draft.RenderYAML()
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	// Intentionally break the Target's profileRef so the loader must reject it.
	broken := strings.Replace(string(targetYAML), "profileRef: deployment-images", "profileRef: does-not-exist", 1)
	if broken == string(targetYAML) {
		t.Fatal("test setup: failed to corrupt profileRef in target YAML")
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, initProfileFileName), profileYAML, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, initTargetFileName), []byte(broken), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	err = ValidateInitConfig(outDir)
	if err == nil {
		t.Fatal("expected ValidateInitConfig to fail on invalid profileRef, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "profileRef") || !strings.Contains(msg, "does-not-exist") {
		t.Fatalf("error must be actionable about profileRef, got: %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
