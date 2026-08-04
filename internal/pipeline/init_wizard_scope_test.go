// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func writeInitTestKubeconfig(t *testing.T) string {
	t.Helper()
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["dev"] = &clientcmdapi.Cluster{Server: "https://example.invalid:6443"}
	cfg.AuthInfos["dev"] = clientcmdapi.NewAuthInfo()
	cfg.Contexts["dev"] = &clientcmdapi.Context{Cluster: "dev", AuthInfo: "dev"}
	cfg.CurrentContext = "dev"
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*cfg, path); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func deploymentResource() InitResourceInfo {
	return InitResourceInfo{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Resource: "deployments", Namespaced: true, Verbs: []string{"list"},
	}
}

func TestMatchInitPattern(t *testing.T) {
	t.Parallel()

	names := []string{"team-a", "team-b", "kube-system", "default"}
	got := matchInitPattern(names, "team-*")
	want := []string{"team-a", "team-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := matchInitPattern(names, "no-such-*"); len(got) != 0 {
		t.Fatalf("zero-match want empty, got %v", got)
	}
}

func TestParseInitMatchLabels(t *testing.T) {
	t.Parallel()

	got, err := parseInitMatchLabels("team=platform,env=prod")
	if err != nil {
		t.Fatal(err)
	}
	if got["team"] != "platform" || got["env"] != "prod" {
		t.Fatalf("got %#v", got)
	}
	if _, err := parseInitMatchLabels(""); err == nil {
		t.Fatal("expected error for empty selector")
	}
	if _, err := parseInitMatchLabels("not-a-pair"); err == nil {
		t.Fatal("expected error for malformed selector")
	}
}

func TestCollectInitNamePattern_zeroMatchRefused(t *testing.T) {
	t.Parallel()

	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeNamePattern},
		{Input: "no-such-*"},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources:  []InitResourceInfo{deploymentResource()},
			Namespaces: []string{"default", "team-a"},
		},
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected zero-match pattern to be refused")
	}
	if !strings.Contains(err.Error(), "matched no namespaces") &&
		!strings.Contains(err.Error(), "no namespaces") {
		t.Fatalf("error should mention zero match, got: %v", err)
	}
}

func TestCollectInitNamePattern_writesIncludedNamespaces(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	var stderr bytes.Buffer
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeNamePattern},
		{Input: "team-*"},
		{Confirm: boolPtrInit(false)}, // keep snapshot; decline durable selector
		{Select: InitFilterNone},
		{MultiSelect: []string{"name"}},
		{Input: "deps"},
		{Confirm: boolPtrInit(true)},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources:  []InitResourceInfo{deploymentResource()},
			Namespaces: []string{"default", "team-a", "team-b"},
		},
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "target.yaml")) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "includedNamespaces:") {
		t.Fatalf("pattern scope must emit includedNamespaces, got:\n%s", text)
	}
	if !strings.Contains(text, "team-a") || !strings.Contains(text, "team-b") {
		t.Fatalf("expected matched namespaces in YAML:\n%s", text)
	}
	// Truthfulness: Target API has no glob field — the pattern must not be written as if it
	// retains glob semantics (REQ-PIPE-07 / B7).
	if strings.Contains(text, "team-*") {
		t.Fatalf("generated YAML must not retain the discovery pattern as a glob:\n%s", text)
	}
	if strings.Contains(text, "namespaceSelector:") {
		t.Fatalf("declined durable path must not emit namespaceSelector:\n%s", text)
	}
	out := stderr.String()
	if !strings.Contains(out, "not auto-included") && !strings.Contains(out, "snapshot") {
		t.Fatalf("wizard must warn the match is a non-durable snapshot, stderr:\n%s", out)
	}
	if !strings.Contains(out, "namespaceSelector") {
		t.Fatalf("wizard must recommend namespaceSelector for durable membership, stderr:\n%s", out)
	}
}

// TestCollectInitNamePattern_switchesToDurableSelector covers the B7 recommend/generate path:
// when the operator wants durable dynamic membership, the wizard switches to namespaceSelector
// instead of leaving a snapshot includedNamespaces list that looks like a glob.
func TestCollectInitNamePattern_switchesToDurableSelector(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeNamePattern},
		{Input: "team-*"},
		{Confirm: boolPtrInit(true)}, // accept durable namespaceSelector
		{Input: "team=platform"},
		{Select: InitFilterNone},
		{MultiSelect: []string{"name"}},
		{Input: "deps"},
		{Confirm: boolPtrInit(true)},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources:  []InitResourceInfo{deploymentResource()},
			Namespaces: []string{"default", "team-a", "team-b"},
		},
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "target.yaml")) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "namespaceSelector:") || !strings.Contains(text, "team: platform") {
		t.Fatalf("durable path must emit namespaceSelector matchLabels, got:\n%s", text)
	}
	if strings.Contains(text, "includedNamespaces:") {
		t.Fatalf("durable path must not keep snapshot includedNamespaces:\n%s", text)
	}
	if strings.Contains(text, "team-*") {
		t.Fatalf("generated YAML must not retain the discovery pattern as a glob:\n%s", text)
	}
}

func TestCollectInitExplicitNamespaces_emptyPickRefused(t *testing.T) {
	t.Parallel()

	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeExplicit},
		{MultiSelect: []string{}}, // empty selection
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources:  []InitResourceInfo{deploymentResource()},
			Namespaces: []string{"default", "team-a"},
		},
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected empty explicit namespace pick to be refused")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "namespace") {
		t.Fatalf("error should mention namespaces, got: %v", err)
	}
}

func TestCollectInitExplicitNamespaces_writesIncludedNamespaces(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeExplicit},
		{MultiSelect: []string{"team-a"}},
		{Select: InitFilterNone},
		{MultiSelect: []string{"name"}},
		{Input: "deps"},
		{Confirm: boolPtrInit(true)},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources:  []InitResourceInfo{deploymentResource()},
			Namespaces: []string{"default", "team-a"},
		},
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "target.yaml")) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "includedNamespaces:") || !strings.Contains(text, "team-a") {
		t.Fatalf("explicit scope must emit includedNamespaces:\n%s", text)
	}
	if strings.Contains(text, "default") {
		t.Fatalf("should not include unselected namespaces:\n%s", text)
	}
}

func TestSelectInitResource_discoveryFailure(t *testing.T) {
	t.Parallel()

	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{Err: errors.New("forbidden")},
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected discovery failure")
	}
	if !strings.Contains(err.Error(), "API discovery") &&
		!strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("want discovery/RBAC error, got: %v", err)
	}
}

func TestSelectInitResource_noListableResources(t *testing.T) {
	t.Parallel()

	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{
			Resources: []InitResourceInfo{
				{Group: "", Version: "v1", Kind: "Pod", Resource: "pods",
					Namespaced: true, Verbs: []string{"watch"}}, // not list/get
			},
		},
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected no-listable error")
	}
	if !strings.Contains(err.Error(), "listable") {
		t.Fatalf("want listable message, got: %v", err)
	}
}

func TestCollectInitExplicitNamespaces_listNamespacesFailure(t *testing.T) {
	t.Parallel()

	// Discoverer returns resources OK, but ListNamespaces fails when scope needs it.
	disc := &failingNSDiscoverer{
		resources: []InitResourceInfo{deploymentResource()},
		nsErr:     errors.New("namespaces forbidden"),
	}
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)},
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeExplicit},
	})
	var stderr bytes.Buffer
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: disc,
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err == nil {
		t.Fatal("expected list namespaces failure")
	}
	if !strings.Contains(err.Error(), "namespaces") {
		t.Fatalf("want namespace list error, got: %v", err)
	}
}

type failingNSDiscoverer struct {
	resources []InitResourceInfo
	nsErr     error
}

func (f *failingNSDiscoverer) ListResources(context.Context) ([]InitResourceInfo, error) {
	return append([]InitResourceInfo(nil), f.resources...), nil
}

func (f *failingNSDiscoverer) ListNamespaces(context.Context) ([]string, error) {
	return nil, f.nsErr
}

func boolPtrInit(v bool) *bool { return &v }
