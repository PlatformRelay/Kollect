// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

type recordedExport struct {
	path    string
	payload []byte
}

type mockBackend struct {
	exports []recordedExport
	err     error
}

func (m *mockBackend) Type() string { return "mock" }

func (m *mockBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (m *mockBackend) Export(_ context.Context, payload []byte, path string) error {
	if m.err != nil {
		return m.err
	}

	m.exports = append(m.exports, recordedExport{path: path, payload: payload})

	return nil
}

func storeWithTarget(namespace, name string) *collect.Store {
	s := collect.NewStore()
	s.Upsert(collect.Item{
		TargetNamespace: namespace,
		TargetName:      name,
		Namespace:       namespace,
		Name:            "item-1",
		Kind:            "Secret",
		UID:             "uid-1",
		Attributes:      map[string]any{"chart": "myapp-1.0.0"},
	})

	return s
}

func TestExportTargets_allSucceed(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{TargetNamespace: "default", TargetName: "t1", Namespace: "default", Name: "a", UID: "1"})
	store.Upsert(collect.Item{TargetNamespace: "default", TargetName: "t2", Namespace: "default", Name: "b", UID: "2"})

	targets := []kollectdevv1alpha1.KollectTarget{testTarget("default", "t1"), testTarget("default", "t2")}
	backend := &mockBackend{}

	exported, errs := ExportTargets(context.Background(), store, targets, backend,
		kollectdevv1alpha1.KollectSinkSpec{}, "", false)

	if exported != 2 {
		t.Errorf("exported = %d, want 2", exported)
	}
	if len(errs) != 0 {
		t.Errorf("errs = %v, want empty", errs)
	}
	if len(backend.exports) != 2 {
		t.Errorf("backend recorded %d exports, want 2", len(backend.exports))
	}
}

func TestExportTargets_dryRunDoesNotCallBackend(t *testing.T) {
	t.Parallel()

	store := storeWithTarget("default", "t1")
	targets := []kollectdevv1alpha1.KollectTarget{testTarget("default", "t1")}
	backend := &mockBackend{}

	exported, errs := ExportTargets(context.Background(), store, targets, backend,
		kollectdevv1alpha1.KollectSinkSpec{}, "", true)

	if exported != 0 {
		t.Errorf("exported = %d, want 0 in dry-run", exported)
	}
	if len(errs) != 0 {
		t.Errorf("errs = %v, want empty", errs)
	}
	if len(backend.exports) != 0 {
		t.Errorf("backend.Export was called %d times, want 0 in dry-run", len(backend.exports))
	}
}

func TestExportTargets_backendErrorIsCollected(t *testing.T) {
	t.Parallel()

	store := storeWithTarget("default", "t1")
	target2 := testTarget("default", "t2")
	store.Upsert(collect.Item{TargetNamespace: "default", TargetName: "t2", Namespace: "default", Name: "b", UID: "2"})

	targets := []kollectdevv1alpha1.KollectTarget{testTarget("default", "t1"), target2}
	backend := &mockBackend{err: errors.New("disk full")}

	exported, errs := ExportTargets(context.Background(), store, targets, backend,
		kollectdevv1alpha1.KollectSinkSpec{}, "", false)

	if exported != 0 {
		t.Errorf("exported = %d, want 0", exported)
	}
	if len(errs) != 2 {
		t.Fatalf("errs = %v, want 2 (one per target, no short-circuit)", errs)
	}
}

func TestExportTargets_pathTemplateRendered(t *testing.T) {
	t.Parallel()

	store := storeWithTarget("app", "inv")
	targets := []kollectdevv1alpha1.KollectTarget{testTarget("app", "inv")}
	backend := &mockBackend{}

	sinkSpec := kollectdevv1alpha1.KollectSinkSpec{
		PathTemplate: "{cluster}/{namespace}/{name}.yaml",
		Cluster:      "prod",
	}

	_, errs := ExportTargets(context.Background(), store, targets, backend, sinkSpec, "", false)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if len(backend.exports) != 1 {
		t.Fatalf("expected 1 export, got %d", len(backend.exports))
	}
	if backend.exports[0].path != "prod/app/inv.yaml" {
		t.Errorf("path = %q, want %q", backend.exports[0].path, "prod/app/inv.yaml")
	}
}

func TestExportTargets_clusterTemplateDefaultsToContextNameWhenSpecClusterUnset(t *testing.T) {
	t.Parallel()

	store := storeWithTarget("app", "inv")
	targets := []kollectdevv1alpha1.KollectTarget{testTarget("app", "inv")}
	backend := &mockBackend{}

	sinkSpec := kollectdevv1alpha1.KollectSinkSpec{PathTemplate: "{cluster}/{namespace}/{name}.yaml"}

	_, errs := ExportTargets(context.Background(), store, targets, backend, sinkSpec, "prod-eu-1", false)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if backend.exports[0].path != "prod-eu-1/app/inv.yaml" {
		t.Errorf("path = %q, want %q", backend.exports[0].path, "prod-eu-1/app/inv.yaml")
	}
}

func testTarget(namespace, name string) kollectdevv1alpha1.KollectTarget {
	tgt := kollectdevv1alpha1.KollectTarget{}
	tgt.Namespace = namespace
	tgt.Name = name

	return tgt
}

func TestResolveSink_outputImpliesLocalSink(t *testing.T) {
	t.Parallel()

	spec, err := ResolveSink(LoadResult{}, "/tmp/out")
	if err != nil {
		t.Fatalf("ResolveSink() error = %v", err)
	}
	if spec.Type != LocalSinkType || spec.Endpoint != "/tmp/out" {
		t.Errorf("spec = %+v, want type=%s endpoint=/tmp/out", spec, LocalSinkType)
	}
}

func TestResolveSink_outputAndSinkYAMLAreAmbiguous(t *testing.T) {
	t.Parallel()

	loaded := LoadResult{Sinks: []kollectdevv1alpha1.KollectSnapshotSink{{}}}

	_, err := ResolveSink(loaded, "/tmp/out")
	if err == nil {
		t.Fatal("expected error for --output + Sink YAML ambiguity, got nil")
	}
}

func TestResolveSink_zeroSinksNoOutputIsError(t *testing.T) {
	t.Parallel()

	_, err := ResolveSink(LoadResult{}, "")
	if err == nil {
		t.Fatal("expected error for zero sinks and no --output, got nil")
	}
}

func TestResolveSink_multipleSinksIsError(t *testing.T) {
	t.Parallel()

	loaded := LoadResult{Sinks: []kollectdevv1alpha1.KollectSnapshotSink{{}, {}}}

	_, err := ResolveSink(loaded, "")
	if err == nil {
		t.Fatal("expected error for multiple sinks, got nil")
	}
}

func TestResolveSink_singleSinkUsesItsSpec(t *testing.T) {
	t.Parallel()

	snap := kollectdevv1alpha1.KollectSnapshotSink{}
	snap.Spec.Type = "git"
	snap.Spec.Endpoint = "https://example.invalid/repo.git"

	spec, err := ResolveSink(LoadResult{Sinks: []kollectdevv1alpha1.KollectSnapshotSink{snap}}, "")
	if err != nil {
		t.Fatalf("ResolveSink() error = %v", err)
	}
	if spec.Type != "git" || spec.Endpoint != "https://example.invalid/repo.git" {
		t.Errorf("spec = %+v, want the loaded sink's normalized spec", spec)
	}
}

func TestResolveSinkSecretData_noSecretRefReturnsNil(t *testing.T) {
	t.Parallel()

	data, err := ResolveSinkSecretData(kollectdevv1alpha1.KollectSinkSpec{}, nil)
	if err != nil {
		t.Fatalf("ResolveSinkSecretData() error = %v", err)
	}
	if data != nil {
		t.Errorf("data = %v, want nil", data)
	}
}

func TestResolveSinkSecretData_foundReturnsData(t *testing.T) {
	t.Parallel()

	secret := corev1.Secret{}
	secret.Name = "sink-creds"
	secret.Namespace = "default"
	secret.Data = map[string][]byte{"token": []byte("shh")}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "sink-creds", Namespace: "default"},
	}

	data, err := ResolveSinkSecretData(spec, []corev1.Secret{secret})
	if err != nil {
		t.Fatalf("ResolveSinkSecretData() error = %v", err)
	}
	if string(data["token"]) != "shh" {
		t.Errorf("data[token] = %q, want shh", data["token"])
	}
}

func TestResolveSinkSecretData_notFoundReturnsError(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing"},
	}

	_, err := ResolveSinkSecretData(spec, nil)
	if err == nil {
		t.Fatal("expected error for unresolved secretRef, got nil")
	}
}

func TestApplyNamespaceOverride_setsIncludedNamespaces(t *testing.T) {
	t.Parallel()

	tgt := testTarget("default", "t1")
	tgt.Spec.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"team": "payments"}}
	tgt.Spec.IncludedNamespaces = []string{"other-ns"}

	got := ApplyNamespaceOverride([]kollectdevv1alpha1.KollectTarget{tgt}, "emb-test")

	if len(got) != 1 {
		t.Fatalf("got %d targets, want 1", len(got))
	}
	if diff := got[0].Spec.IncludedNamespaces; len(diff) != 1 || diff[0] != "emb-test" {
		t.Errorf("IncludedNamespaces = %v, want [emb-test]", diff)
	}
}

func TestApplyNamespaceOverride_emptyNamespaceIsNoop(t *testing.T) {
	t.Parallel()

	tgt := testTarget("default", "t1")
	tgt.Spec.IncludedNamespaces = []string{"other-ns"}

	got := ApplyNamespaceOverride([]kollectdevv1alpha1.KollectTarget{tgt}, "")

	if len(got) != 1 || len(got[0].Spec.IncludedNamespaces) != 1 || got[0].Spec.IncludedNamespaces[0] != "other-ns" {
		t.Errorf("expected targets unchanged when namespace override is empty, got %+v", got)
	}
}

func TestBuildContextResult_foldsSkippedTargetsIntoResult(t *testing.T) {
	t.Parallel()

	runResult := collect.RunResult{
		ItemCount:      0,
		SkippedTargets: []collect.SkippedTarget{{Name: "default/t1", Reason: "forbidden"}},
	}

	got := buildContextResult("ctx-a", runResult, 0, nil)

	if got.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", got.Skipped)
	}
	if got.Exported != 0 {
		t.Errorf("Exported = %d, want 0", got.Exported)
	}
}

func TestBuildContextResult_combinesRunAndExportErrors(t *testing.T) {
	t.Parallel()

	runResult := collect.RunResult{Errors: []error{errors.New("namespace list failed")}}
	exportErrs := []error{errors.New("export failed")}

	got := buildContextResult("ctx-a", runResult, 1, exportErrs)

	if len(got.Errs) != 2 {
		t.Fatalf("Errs = %v, want 2 combined errors", got.Errs)
	}
}

// TestRestConfigForContext_selectsNamedContextsServer guards against a real bug: passing
// a context name as clientcmd.BuildConfigFromFlags' first argument (masterUrl) silently
// ignores context selection and treats the context name as a server hostname override.
// restConfigForContext must select each context's own cluster server.
func TestRestConfigForContext_selectsNamedContextsServer(t *testing.T) {
	t.Parallel()

	path := writeMultiClusterFixtureKubeconfig(t)

	cfgA, err := restConfigForContext(path, "ctx-a")
	if err != nil {
		t.Fatalf("restConfigForContext(ctx-a) error = %v", err)
	}
	if cfgA.Host != "https://server-a.example.invalid:6443" {
		t.Errorf("ctx-a Host = %q, want https://server-a.example.invalid:6443", cfgA.Host)
	}

	cfgB, err := restConfigForContext(path, "ctx-b")
	if err != nil {
		t.Fatalf("restConfigForContext(ctx-b) error = %v", err)
	}
	if cfgB.Host != "https://server-b.example.invalid:6443" {
		t.Errorf("ctx-b Host = %q, want https://server-b.example.invalid:6443", cfgB.Host)
	}
}

func writeMultiClusterFixtureKubeconfig(t *testing.T) string {
	t.Helper()

	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://server-a.example.invalid:6443
- name: cluster-b
  cluster:
    server: https://server-b.example.invalid:6443
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
current-context: ctx-a
users:
- name: user-a
  user:
    token: fake-token-a
- name: user-b
  user:
    token: fake-token-b
`
	dir := t.TempDir()
	path := dir + "/kubeconfig"
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}
