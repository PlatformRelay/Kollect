// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/konih/kollect/internal/sink"
)

// TestPipelineCLI_collectAndWrite is the L2 integration tier for the pipeline CLI (ADR-0801,
// P-008): it drives the real top-level orchestration entry point (RunAllContexts, i.e. the same
// LoadConfig -> per-context runner -> store -> ExportTargets wiring the `collect` command runs)
// against a live envtest API server, then asserts one output file per target with the extracted
// attributes present. Driving RunAllContexts (rather than composing NewRunner by hand) exercises
// the real kubeconfig-context resolution, client construction, and sink wiring end to end.
//
// It runs in `task test` (the Makefile sets KUBEBUILDER_ASSETS via setup-envtest). It is *not*
// meant to run in the `tags=integration` job, which sets up testcontainers but no envtest
// binaries; there it skips cleanly rather than failing on a missing API server. The guard keys
// on asset availability only, so the standard test job always exercises it for real.
func TestPipelineCLI_collectAndWrite(t *testing.T) {
	assetsDir := pipelineEnvtestAssetsDir()
	if assetsDir == "" {
		t.Skip("envtest assets unavailable (KUBEBUILDER_ASSETS unset and no bin/k8s); L2 runs in `task test`, not the tags=integration job")
	}

	testEnv := &envtest.Environment{BinaryAssetsDirectory: assetsDir}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("envtest start: %v", err)
	}

	t.Cleanup(func() {
		if stopErr := testEnv.Stop(); stopErr != nil {
			t.Errorf("envtest stop: %v", stopErr)
		}
	})

	ctx := context.Background()

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}

	const namespace = "pipeline-it"
	seedEnvtestObjects(ctx, t, kube, namespace)

	loaded, err := LoadConfig("testdata/envtest")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(loaded.Targets) != 2 {
		t.Fatalf("loaded %d targets, want 2 (deployment + ingress)", len(loaded.Targets))
	}

	outDir := t.TempDir()

	sinkSpec, err := ResolveSink(loaded, outDir)
	if err != nil {
		t.Fatalf("resolve sink: %v", err)
	}

	const contextName = "envtest"
	kubeconfigPath := writeEnvtestKubeconfig(t, cfg, contextName)

	results := RunAllContexts(ctx, []string{contextName}, kubeconfigPath, loaded, sinkSpec, nil, sink.NewRegistry(), nil, false)
	if len(results) != 1 {
		t.Fatalf("got %d context results, want 1", len(results))
	}

	res := results[0]
	if res.Fatal != nil {
		t.Fatalf("context fatal: %v", res.Fatal)
	}
	if len(res.Errs) != 0 {
		t.Fatalf("context errors: %v", res.Errs)
	}
	if res.Skipped != 0 {
		t.Fatalf("targets skipped: %d", res.Skipped)
	}
	if res.Exported != 2 {
		t.Fatalf("exported = %d, want 2 (one per target)", res.Exported)
	}

	// One file per target, no more (default pathTemplate: inventory/{namespace}/{name}.yaml,
	// where {namespace} is the KollectTarget's namespace).
	if got := countFiles(t, outDir); got != 2 {
		t.Fatalf("output dir has %d files, want exactly 2", got)
	}

	deployFile := filepath.Join(outDir, "inventory", namespace, "deploy-target.yaml")
	ingressFile := filepath.Join(outDir, "inventory", namespace, "ingress-target.yaml")

	// The deployment envelope must carry the collected image attribute.
	assertEnvelope(t, deployFile, "nginx:1.25")
	// The ingress envelope must be a valid, non-empty export (host attribute is optional).
	assertEnvelope(t, ingressFile, "")
}

// seedEnvtestObjects creates the namespace plus one Deployment and one Ingress that the two
// profiles collect. Uses only built-in types, so no CRDs need to be installed in the apiserver.
func seedEnvtestObjects(ctx context.Context, t *testing.T, kube kubernetes.Interface, namespace string) {
	t.Helper()

	if _, err := kube.CoreV1().Namespaces().Create(ctx,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create namespace: %v", err)
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "web", Image: "nginx:1.25"}},
				},
			},
		},
	}
	if _, err := kube.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "example.com"}},
		},
	}
	if _, err := kube.NetworkingV1().Ingresses(namespace).Create(ctx, ing, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create ingress: %v", err)
	}
}

// writeEnvtestKubeconfig renders a kubeconfig file pointing at the envtest API server so the
// pipeline's kubeconfig-based context resolution (restConfigForContext) can be exercised end to
// end. envtest authenticates via client cert/key, which map onto the kubeconfig AuthInfo.
func writeEnvtestKubeconfig(t *testing.T, cfg *rest.Config, contextName string) string {
	t.Helper()

	apiCfg := clientcmdapi.NewConfig()
	apiCfg.Clusters["envtest"] = &clientcmdapi.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: cfg.CAData,
	}
	apiCfg.AuthInfos["envtest"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
	}
	apiCfg.Contexts[contextName] = &clientcmdapi.Context{Cluster: "envtest", AuthInfo: "envtest"}
	apiCfg.CurrentContext = contextName

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*apiCfg, path); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	return path
}

// assertEnvelope parses the export envelope at path (JSON payload written to a .yaml path by the
// local sink) and asserts it has at least one item; when want is non-empty the raw payload must
// contain it (e.g. a collected attribute value).
func assertEnvelope(t *testing.T, path, want string) {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is composed from the test's own t.TempDir()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var env struct {
		ItemCount int `json:"itemCount"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse envelope %s: %v", path, err)
	}
	if env.ItemCount < 1 {
		t.Errorf("%s itemCount = %d, want >= 1", path, env.ItemCount)
	}
	if want != "" && !strings.Contains(string(raw), want) {
		t.Errorf("%s does not contain %q; payload=%s", path, want, raw)
	}
}

func countFiles(t *testing.T, root string) int {
	t.Helper()

	count := 0
	if err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}

		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return count
}

// pipelineEnvtestAssetsDir resolves the envtest binary assets directory the same way the collect
// scale test does: KUBEBUILDER_ASSETS first (set by the Makefile `test` target), then a local
// bin/k8s/<version> directory. Returns "" when neither is present so the caller can skip.
func pipelineEnvtestAssetsDir() string {
	if assets := os.Getenv("KUBEBUILDER_ASSETS"); assets != "" {
		if abs, err := filepath.Abs(assets); err == nil {
			return abs
		}

		return assets
	}

	for _, basePath := range []string{
		filepath.Join("bin", "k8s"),
		filepath.Join("..", "..", "bin", "k8s"),
	} {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				if abs, err := filepath.Abs(filepath.Join(basePath, entry.Name())); err == nil {
					return abs
				}
			}
		}
	}

	return ""
}
