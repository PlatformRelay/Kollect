// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/platformrelay/kollect/internal/validation"
)

func secretResource() InitResourceInfo {
	return InitResourceInfo{
		Group: "", Version: "v1", Kind: "Secret",
		Resource: "secrets", Namespaced: true, Verbs: []string{"list", "get"},
	}
}

// recordingSampler fails the test if GetSampleObject is called when forbidden,
// and records every Get for consent assertions.
type recordingSampler struct {
	t          *testing.T
	candidates []InitSampleRef
	object     map[string]any
	allowGet   atomic.Bool
	gets       atomic.Int32
	listCalls  atomic.Int32
	listErr    error
	getErr     error
}

func (s *recordingSampler) ListSampleCandidates(
	_ context.Context, _ InitResourceInfo, _ []string, _ int,
) ([]InitSampleRef, error) {
	s.listCalls.Add(1)
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]InitSampleRef, len(s.candidates))
	copy(out, s.candidates)
	return out, nil
}

func (s *recordingSampler) GetSampleObject(_ context.Context, ref InitSampleRef) (map[string]any, error) {
	s.gets.Add(1)
	if !s.allowGet.Load() {
		s.t.Fatalf("GetSampleObject(%+v) called without consent gate allowing a read", ref)
	}
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.object, nil
}

func TestInitAttributeSampling_declinedKeepsSafeDefaultsNoRead(t *testing.T) {
	t.Parallel()

	sampler := &recordingSampler{
		t: t,
		candidates: []InitSampleRef{{
			Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments",
			Namespace: "default", Name: "api",
		}},
		object: map[string]any{
			"metadata": map[string]any{"name": "api", "namespace": "default"},
			"spec":     map[string]any{"replicas": float64(2)},
		},
	}
	// Declining sampling must never enable Get.
	sampler.allowGet.Store(false)

	outDir := t.TempDir()
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)}, // kubecontext
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeAll},
		{Select: InitFilterNone},
		{Confirm: boolPtrInit(false)}, // decline sampling
		{MultiSelect: []string{"name"}},
		{Input: "deps"},
		{Confirm: boolPtrInit(true)}, // write
	})
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{Resources: []InitResourceInfo{deploymentResource()}},
		Sampler:    sampler,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if sampler.gets.Load() != 0 {
		t.Fatalf("declined sampling must not read objects, gets=%d", sampler.gets.Load())
	}
	if sampler.listCalls.Load() != 0 {
		t.Fatalf("declined sampling must not list candidates, lists=%d", sampler.listCalls.Load())
	}
}

func TestInitAttributeSampling_requiresIdentityConsentBeforeRead(t *testing.T) {
	t.Parallel()

	ref := InitSampleRef{
		Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments",
		Namespace: "default", Name: "api",
	}
	sampler := &recordingSampler{
		t:          t,
		candidates: []InitSampleRef{ref},
		object: map[string]any{
			"metadata": map[string]any{"name": "api", "namespace": "default", "labels": map[string]any{"app": "api"}},
			"spec":     map[string]any{"replicas": float64(3)},
		},
	}
	// Get is only allowed after the identity consent prompt is answered yes.
	// ScriptedPrompter runs synchronously; we flip allowGet in a Confirm that
	// matches the identity prompt by using a custom Prompter wrapper below.
	base := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)}, // kubecontext
		{Select: "Deployment (apps/v1)"},
		{Select: InitScopeAll},
		{Select: InitFilterNone},
		{Confirm: boolPtrInit(true)}, // consent to sample
		{Select: ref.Label()},        // pick candidate
		{Confirm: boolPtrInit(true)}, // identity consent — gate opens here
		{MultiSelect: []string{"name", "replicas"}},
		{Input: "deps"},
		{Confirm: boolPtrInit(true)},
	})
	prompter := &consentGatePrompter{inner: base, sampler: sampler, needle: "Read object"}

	var stderr bytes.Buffer
	_, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   prompter,
		Discoverer: &FakeInitDiscoverer{Resources: []InitResourceInfo{deploymentResource()}},
		Sampler:    sampler,
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v\nstderr:\n%s", err, stderr.String())
	}
	if sampler.gets.Load() != 1 {
		t.Fatalf("expected exactly one consented Get, got %d", sampler.gets.Load())
	}
	out := stderr.String()
	if !strings.Contains(out, "Deployment") || !strings.Contains(out, "default") || !strings.Contains(out, "api") {
		t.Fatalf("identity (GVK/namespace/name) must be shown before read, stderr:\n%s", out)
	}
	if !strings.Contains(out, "apps") && !strings.Contains(out, "apps/v1") {
		// Label() includes group/version; Identity line should too.
		if !strings.Contains(out, ref.Identity()) && !strings.Contains(out, "api") {
			t.Fatalf("expected sample identity in stderr:\n%s", out)
		}
	}
}

func TestInitAttributeSampling_secretDeniedWithoutDistinctGuard(t *testing.T) {
	t.Parallel()

	secretVal := "super-secret-token-value"
	ref := InitSampleRef{
		Group: "", Version: "v1", Kind: "Secret", Resource: "secrets",
		Namespace: "default", Name: "db",
	}
	sampler := &recordingSampler{
		t:          t,
		candidates: []InitSampleRef{ref},
		object: map[string]any{
			"metadata": map[string]any{"name": "db", "namespace": "default"},
			"data":     map[string]any{"password": secretVal},
		},
	}
	sampler.allowGet.Store(false)

	var stderr bytes.Buffer
	script := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)}, // kubecontext
		{Select: "Secret (v1)"},
		{Select: InitScopeAll},
		{Select: InitFilterNone},
		{Confirm: boolPtrInit(true)},  // want to sample
		{Confirm: boolPtrInit(false)}, // decline sensitive-kind guard
		{MultiSelect: []string{"name"}},
		{Input: "secrets"},
		{Confirm: boolPtrInit(true)},
	})
	res, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  t.TempDir(),
		Prompter:   script,
		Discoverer: &FakeInitDiscoverer{Resources: []InitResourceInfo{secretResource()}},
		Sampler:    sampler,
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if sampler.gets.Load() != 0 {
		t.Fatalf("declined sensitive guard must not read Secret objects, gets=%d", sampler.gets.Load())
	}
	if strings.Contains(stderr.String(), secretVal) {
		t.Fatalf("secret values must never appear in output, stderr:\n%s", stderr.String())
	}
	raw, err := os.ReadFile(res.ProfilePath) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), validation.AllowSecretExtractionAnnotation) {
		t.Fatalf("declined sensitive guard must not emit secret opt-in annotation:\n%s", raw)
	}
}

func TestInitAttributeSampling_secretGuardThenConsentAddsOptInNeverPrintsValues(t *testing.T) {
	t.Parallel()

	secretVal := "super-secret-token-value"
	ref := InitSampleRef{
		Group: "", Version: "v1", Kind: "Secret", Resource: "secrets",
		Namespace: "default", Name: "db",
	}
	sampler := &recordingSampler{
		t:          t,
		candidates: []InitSampleRef{ref},
		object: map[string]any{
			"metadata": map[string]any{"name": "db", "namespace": "default"},
			"data":     map[string]any{"password": secretVal},
		},
	}
	base := NewScriptedPrompter([]PromptAnswer{
		{Confirm: boolPtrInit(true)}, // kubecontext
		{Select: "Secret (v1)"},
		{Select: InitScopeAll},
		{Select: InitFilterNone},
		{Confirm: boolPtrInit(true)}, // want to sample
		{Confirm: boolPtrInit(true)}, // sensitive-kind guard
		{Select: ref.Label()},
		{Confirm: boolPtrInit(true)}, // identity consent
		{MultiSelect: []string{"name", "data.password"}},
		{Input: "secrets"},
		{Confirm: boolPtrInit(true)},
	})
	prompter := &consentGatePrompter{inner: base, sampler: sampler, needle: "Read object"}

	outDir := t.TempDir()
	var stderr bytes.Buffer
	res, err := RunInit(InitOptions{
		Kubeconfig: writeInitTestKubeconfig(t),
		OutputDir:  outDir,
		Prompter:   prompter,
		Discoverer: &FakeInitDiscoverer{Resources: []InitResourceInfo{secretResource()}},
		Sampler:    sampler,
		Stderr:     &stderr,
		IsTerminal: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("RunInit: %v\nstderr:\n%s", err, stderr.String())
	}
	if sampler.gets.Load() != 1 {
		t.Fatalf("expected consented Secret Get, got %d", sampler.gets.Load())
	}
	out := stderr.String()
	if strings.Contains(out, secretVal) {
		t.Fatalf("sampled secret values must never be printed, stderr:\n%s", out)
	}
	raw, err := os.ReadFile(res.ProfilePath) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, AllowSecretExtractionAnnotation) {
		t.Fatalf("proceeding with Secret sampling must emit sensitive-data opt-in annotation:\n%s", text)
	}
	if !strings.Contains(text, AllowSecretExtractionAnnotation+":") {
		t.Fatalf("opt-in annotation key missing:\n%s", text)
	}
	if strings.Contains(text, secretVal) {
		t.Fatalf("profile YAML must not embed sampled secret values:\n%s", text)
	}
}

func TestSuggestAttributesFromSample_hidesSecretValues(t *testing.T) {
	t.Parallel()

	secretVal := "never-print-me"
	obj := map[string]any{
		"metadata": map[string]any{"name": "db", "namespace": "ns"},
		"data":     map[string]any{"token": secretVal},
		"type":     "Opaque",
	}
	opts, preview := suggestAttributesFromSample(obj, true)
	joined := strings.Join(preview, "\n")
	if strings.Contains(joined, secretVal) {
		t.Fatalf("preview must not contain secret value, got:\n%s", joined)
	}
	for _, o := range opts {
		if strings.Contains(o.Name, secretVal) || strings.Contains(o.Path, secretVal) {
			t.Fatalf("suggestion must not embed secret value: %+v", o)
		}
	}
	foundDataKey := false
	for _, o := range opts {
		if o.Path == "$.data.token" || strings.HasSuffix(o.Path, ".data.token") {
			foundDataKey = true
		}
	}
	if !foundDataKey {
		t.Fatalf("expected data key path suggestion without value, got %#v", opts)
	}
}

func TestIsSensitiveInitKind_defaults(t *testing.T) {
	t.Parallel()

	if !isSensitiveInitKind(InitResourceInfo{Kind: "Secret", Group: ""}, nil) {
		t.Fatal("core Secret must be sensitive by default")
	}
	if isSensitiveInitKind(InitResourceInfo{Kind: "ConfigMap", Group: ""}, nil) {
		t.Fatal("ConfigMap must not be sensitive by default")
	}
	extra := []InitSensitiveKind{{Group: "example.com", Kind: "Credential"}}
	if !isSensitiveInitKind(InitResourceInfo{Kind: "Credential", Group: "example.com"}, extra) {
		t.Fatal("configured sensitive kinds must match")
	}
}

// consentGatePrompter opens the sampler Get gate only when the identity Confirm runs.
type consentGatePrompter struct {
	inner   *ScriptedPrompter
	sampler *recordingSampler
	needle  string
}

func (p *consentGatePrompter) Input(message, defaultValue string, validate ValidateFunc) (string, error) {
	return p.inner.Input(message, defaultValue, validate)
}

func (p *consentGatePrompter) Select(message string, options []string) (string, error) {
	return p.inner.Select(message, options)
}

func (p *consentGatePrompter) MultiSelect(message string, options, defaults []string) ([]string, error) {
	return p.inner.MultiSelect(message, options, defaults)
}

func (p *consentGatePrompter) Confirm(message string, defaultValue bool) (bool, error) {
	if strings.Contains(message, p.needle) {
		p.sampler.allowGet.Store(true)
	}
	return p.inner.Confirm(message, defaultValue)
}

func TestInitSampleRef_identity(t *testing.T) {
	t.Parallel()

	ns := InitSampleRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "default", Name: "api",
	}
	id := ns.Identity()
	if !strings.Contains(id, "Deployment") || !strings.Contains(id, "default") || !strings.Contains(id, "api") {
		t.Fatalf("identity = %q", id)
	}
	cluster := InitSampleRef{Group: "", Version: "v1", Kind: "Namespace", Name: "default"}
	cid := cluster.Identity()
	if !strings.Contains(cid, "Namespace") || !strings.Contains(cid, "default") {
		t.Fatalf("cluster identity = %q", cid)
	}
	if strings.Contains(cid, "namespace=") {
		t.Fatalf("cluster-scoped identity must not invent a namespace= field: %q", cid)
	}
}
