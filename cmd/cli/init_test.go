// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/pipeline"
)

func TestRootCmd_initSubcommandRegistered(t *testing.T) {
	t.Parallel()

	root, _ := buildRoot()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected init subcommand to be registered on kollect-pipeline root")
	}
}

func TestInitCmd_helpExitsZero(t *testing.T) {
	t.Parallel()

	cmd, _ := newInitCmd()
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--help) error = %v", err)
	}
}

func TestInitCmd_nonTTYReturnsClearErrorWithSamplesPointer(t *testing.T) {
	t.Parallel()

	cmd, _ := newInitCmd()
	cmd.SetArgs([]string{
		"--kubeconfig", writeFixtureKubeconfig(t),
		"--output-dir", t.TempDir(),
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	prev := initIsTerminal
	initIsTerminal = func() bool { return false }
	t.Cleanup(func() { initIsTerminal = prev })

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-TTY error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "interactive terminal") && !strings.Contains(msg, "TTY") {
		t.Fatalf("error should mention interactive/TTY requirement, got: %v", err)
	}
	if !strings.Contains(msg, "config/samples/pipeline") {
		t.Fatalf("error should point at config/samples/pipeline, got: %v", err)
	}
}

func TestInitWizard_transcriptWritesProfileAndTarget(t *testing.T) {
	t.Parallel()

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

	disc := &pipeline.FakeInitDiscoverer{
		Resources: []pipeline.InitResourceInfo{
			{
				Group: "apps", Version: "v1", Kind: "Deployment",
				Resource: "deployments", Namespaced: true, Verbs: []string{"list", "get"},
			},
			{
				Group: "", Version: "v1", Kind: "Namespace",
				Resource: "namespaces", Namespaced: false, Verbs: []string{"list", "get"},
			},
		},
		Namespaces: []string{"default", "kube-system"},
	}

	var stderr bytes.Buffer
	res, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig,
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: disc,
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
		Color:      true,
	})
	if err != nil {
		t.Fatalf("RunInit() error = %v\nstderr:\n%s", err, stderr.String())
	}
	if res.ProfilePath == "" || res.TargetPath == "" {
		t.Fatalf("expected profile and target paths, got %+v", res)
	}

	steps := script.Prompts()
	wantOrder := []string{
		"confirm:", "select:", "select:", "select:", "multiselect:", "input:", "confirm:",
	}
	if len(steps) < len(wantOrder) {
		t.Fatalf("prompt transcript too short: %v", steps)
	}
	for i, prefix := range wantOrder {
		if !strings.HasPrefix(steps[i], prefix) {
			t.Fatalf("step %d: want prefix %q, got %q (full: %v)", i, prefix, steps[i], steps)
		}
	}

	loaded, err := mustLoadConfig(t, outDir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(loaded.Profiles) != 1 || len(loaded.Targets) != 1 {
		t.Fatalf("want 1 profile + 1 target, got %d profiles, %d targets",
			len(loaded.Profiles), len(loaded.Targets))
	}
	p := loaded.Profiles[0]
	if p.Name != "deployment-images" {
		t.Errorf("profile name = %q, want deployment-images", p.Name)
	}
	if p.Spec.TargetGVK.Kind != "Deployment" || p.Spec.TargetGVK.Group != "apps" {
		t.Errorf("targetGVK = %+v, want apps/v1 Deployment", p.Spec.TargetGVK)
	}
	if loaded.Targets[0].Spec.ProfileRef != "deployment-images" {
		t.Errorf("profileRef = %q", loaded.Targets[0].Spec.ProfileRef)
	}
}

func TestInitWizard_cancelSkipsWrite(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	script := pipeline.NewScriptedPrompter([]pipeline.PromptAnswer{
		{Confirm: boolPtr(false)},
	})

	_, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: writeInitKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &pipeline.FakeInitDiscoverer{},
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if !pipeline.IsInitCanceled(err) {
		t.Fatalf("want canceled error, got %v", err)
	}
	entries, _ := listYAML(t, outDir)
	if len(entries) != 0 {
		t.Fatalf("cancel must not write files, found %v", entries)
	}
}

func TestInitWizard_overwriteProtection(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	kubeconfig := writeInitKubeconfig(t)
	disc := &pipeline.FakeInitDiscoverer{
		Resources: []pipeline.InitResourceInfo{
			{
				Group: "apps", Version: "v1", Kind: "Deployment",
				Resource: "deployments", Namespaced: true, Verbs: []string{"list"},
			},
		},
	}

	happy := func(overwriteConfirms ...bool) *pipeline.ScriptedPrompter {
		answers := make([]pipeline.PromptAnswer, 0, 7+len(overwriteConfirms))
		answers = append(answers,
			pipeline.PromptAnswer{Confirm: boolPtr(true)},
			pipeline.PromptAnswer{Select: "Deployment (apps/v1)"},
			pipeline.PromptAnswer{Select: pipeline.InitScopeAll},
			pipeline.PromptAnswer{Select: pipeline.InitFilterNone},
			pipeline.PromptAnswer{MultiSelect: []string{"name"}},
			pipeline.PromptAnswer{Input: "deployment-images"},
			pipeline.PromptAnswer{Confirm: boolPtr(true)},
		)
		for _, c := range overwriteConfirms {
			answers = append(answers, pipeline.PromptAnswer{Confirm: boolPtr(c)})
		}
		return pipeline.NewScriptedPrompter(answers)
	}

	if _, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig, OutputDir: outDir, Prompter: happy(),
		Discoverer: disc, IsTerminal: func() bool { return true },
	}); err != nil {
		t.Fatalf("first write: %v", err)
	}

	marker := "DO-NOT-CLOBBER"
	if err := writeMarker(t, outDir, "profile.yaml", marker); err != nil {
		t.Fatal(err)
	}

	_, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig, OutputDir: outDir, Prompter: happy(false),
		Discoverer: disc, IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected overwrite-declined error")
	}
	raw := readFile(t, outDir, "profile.yaml")
	if !strings.Contains(raw, marker) {
		t.Fatalf("declined overwrite must leave existing file intact, got:\n%s", raw)
	}

	if _, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig, OutputDir: outDir, Prompter: happy(true, true),
		Discoverer: disc, IsTerminal: func() bool { return true },
	}); err != nil {
		t.Fatalf("overwrite accept: %v", err)
	}
	raw = readFile(t, outDir, "profile.yaml")
	if strings.Contains(raw, marker) {
		t.Fatal("accepted overwrite must replace existing file")
	}
	if !strings.Contains(raw, "kind: KollectProfile") {
		t.Fatalf("expected KollectProfile YAML, got:\n%s", raw)
	}
}

func TestInitWizard_noColorHonored(t *testing.T) {
	t.Parallel()

	script := pipeline.NewScriptedPrompter([]pipeline.PromptAnswer{
		{Confirm: boolPtr(true)},
		{Select: "Namespace (v1)"},
		{Select: pipeline.InitFilterNone},
		{MultiSelect: []string{"name"}},
		{Input: "namespaces"},
		{Confirm: boolPtr(true)},
	})
	var stderr bytes.Buffer
	_, err := pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: writeInitKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &pipeline.FakeInitDiscoverer{
			Resources: []pipeline.InitResourceInfo{
				{
					Group: "", Version: "v1", Kind: "Namespace",
					Resource: "namespaces", Namespaced: false, Verbs: []string{"list"},
				},
			},
		},
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
		Color:      false,
	})
	if err != nil {
		t.Fatalf("RunInit() error = %v", err)
	}
	out := stderr.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("NO_COLOR/plain mode must not emit ANSI escapes, got %q", out)
	}
}

func boolPtr(v bool) *bool { return &v }
