// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/pipeline"
)

// TestInitWizard_generatedYAMLPassesLoaderAndStdoutTrial (PIPE-INIT-04 / REQ-PIPE-08):
// after a successful init, the written Profile+Target must LoadConfig cleanly and wire
// through `collect --output -` (stdout trial path) without a cluster.
func TestInitWizard_generatedYAMLPassesLoaderAndStdoutTrial(t *testing.T) {
	outDir := t.TempDir()
	kubeconfig := writeInitKubeconfig(t)

	script := pipeline.NewScriptedPrompter([]pipeline.PromptAnswer{
		{Confirm: boolPtr(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: pipeline.InitScopeAll},
		{Select: pipeline.InitFilterNone},
		{MultiSelect: []string{"name", "namespace"}},
		{Input: "deployment-images"},
		{Confirm: boolPtr(true)},
	})

	var stderr bytes.Buffer
	_, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig,
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &pipeline.FakeInitDiscoverer{
			Resources: []pipeline.InitResourceInfo{
				{
					Group: "apps", Version: "v1", Kind: "Deployment",
					Resource: "deployments", Namespaced: true, Verbs: []string{"list", "get"},
				},
			},
		},
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v\nstderr:\n%s", err, stderr.String())
	}

	loaded, err := pipeline.LoadConfig(outDir)
	if err != nil {
		t.Fatalf("LoadConfig(generated): %v", err)
	}
	if len(loaded.Profiles) != 1 || len(loaded.Targets) != 1 {
		t.Fatalf("want 1 profile + 1 target, got %d / %d", len(loaded.Profiles), len(loaded.Targets))
	}

	out := stderr.String()
	if !strings.Contains(out, pipeline.FormatInitTrialStdoutCmd(outDir)) {
		t.Fatalf("completion missing stdout trial command\n%s", out)
	}
	if !strings.Contains(out, pipeline.FormatInitTrialLocalCmd(outDir)) {
		t.Fatalf("completion missing local trial command\n%s", out)
	}

	fakeRunAllContexts(t, pipeline.ContextResult{
		Context:  "dev",
		Exported: 1,
		Records:  []pipeline.StdoutRecord{stdoutRecord("dev", "kollect-system", "deployment-images")},
	})

	cmd, code := newCollectCmd()
	var stdout, errBuf bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--config", outDir,
		"--output", "-",
		"--kubeconfig", kubeconfig,
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if execErr := cmd.Execute(); execErr != nil {
		t.Fatalf("collect --output - with generated config: %v\nstderr:%s", execErr, errBuf.String())
	}
	if *code != ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess", *code)
	}

	var got pipeline.StdoutRecord
	if unmarshalErr := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &got); unmarshalErr != nil {
		t.Fatalf("stdout trial record not NDJSON (%q): %v", stdout.String(), unmarshalErr)
	}
	if got.TargetName != "deployment-images" {
		t.Errorf("stdout trial target = %q, want deployment-images", got.TargetName)
	}

	sinkSpec, err := pipeline.ResolveSink(loaded, pipeline.StdoutSentinel)
	if err != nil {
		t.Fatalf("ResolveSink(stdout) on generated config: %v", err)
	}
	if !pipeline.IsStdoutSink(sinkSpec) {
		t.Fatalf("expected stdout sink, got type %q", sinkSpec.Type)
	}
}
