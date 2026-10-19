// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/envtestassets"
	"github.com/platformrelay/kollect/internal/pipeline"
	"github.com/platformrelay/kollect/internal/sink"
	s3sink "github.com/platformrelay/kollect/internal/sink/s3"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// C1b (openspec/changes/sink-ca-trust SCT-5, "CLI stops before any export"): the real
// `collect` command, against a live envtest API server, with an s3 sink whose CA bundle is
// garbage, collects, then fails the context with the shared CA error and exits 2: the backend
// build fails after collection, so no export runs and nothing reaches the sink. The same
// config with --output exports, so the exit is not from a collection failure.
//
// L2 tier like internal/pipeline's TestPipelineCLI_collectAndWrite: it runs in `task test`
// and skips when no envtest assets are available. Not parallel: it swaps runAllContexts.
func TestCollectCmd_caErrorExitsFatalAfterCollection(t *testing.T) {
	assetsDir := envtestassets.Resolve(
		[]string{filepath.Join("bin", "k8s"), filepath.Join("..", "..", "bin", "k8s")}, runtime.GOOS, runtime.GOARCH)
	if assetsDir == "" {
		t.Skip("envtest assets unavailable (KUBEBUILDER_ASSETS unset and no bin/k8s); L2 runs in `task test`")
	}

	isolateCLIAWSEnv(t)

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

	const namespace = "ca-parity"
	seedCAParityObjects(t, cfg, namespace)

	kubeconfig := writeEnvtestKubeconfigCLI(t, cfg)

	// Spy on the real pipeline: keep each run's results, and count how often the s3 factory
	// is asked for a backend (runOneContext asks only after collection returned).
	var (
		results     []pipeline.ContextResult
		s3Factories int
	)

	orig := runAllContexts
	t.Cleanup(func() { runAllContexts = orig })
	runAllContexts = func(
		ctx context.Context, contexts []string, kubeconfigPath string, loaded pipeline.LoadResult,
		spec kollectdevv1alpha1.KollectSinkSpec, secretData map[string][]byte, registry *sink.Registry,
		scrubKeys []string, dryRun bool,
	) []pipeline.ContextResult {
		stock := sink.NewRegistry()
		registry.Register("s3", func(s kollectdevv1alpha1.KollectSinkSpec, b sink.BuildContext) (sink.Backend, error) {
			s3Factories++

			return stock.NewBackend(s, b)
		})

		results = orig(ctx, contexts, kubeconfigPath, loaded, spec, secretData, registry, scrubKeys, dryRun)

		return results
	}

	baseDir := writeCAParityConfigDir(t, namespace)

	// Control: the same profile and target export through --output.
	outDir := t.TempDir()
	code := runCollectForTest(t, "--config", baseDir, "--kubeconfig", kubeconfig, "--output", outDir)
	if code != ExitSuccess {
		t.Fatalf("control run with --output: exit %d, results %+v; want %d", code, results, ExitSuccess)
	}

	if len(results) != 1 || results[0].Exported != 1 {
		t.Fatalf("control run results %+v, want one context exporting one target", results)
	}

	// The s3 sink with a garbage CA bundle, and with a valid CA and an http endpoint.
	caX := sinktlstest.NewCA(t, "X")

	cases := []struct {
		name, endpoint, caBundle string
		want                     error
	}{
		{
			"garbage CA", "https://sentinel-host.invalid/b",
			base64.StdEncoding.EncodeToString([]byte("not a certificate")), sinktls.ErrNoCACertificate,
		},
		{
			"CA and http endpoint", "http://sentinel-host.invalid/b",
			base64.StdEncoding.EncodeToString(caX.PEM), s3sink.ErrCAPlainHTTP,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sinkDir := writeCAParityConfigDir(t, namespace)
			writeFileForTest(t, filepath.Join(sinkDir, "sink.yaml"), `apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: s3-ca
  namespace: `+namespace+`
spec:
  type: s3
  endpoint: `+tc.endpoint+`
  secretRef:
    name: object-creds
  tls:
    caBundle: `+tc.caBundle+`
`)
			writeFileForTest(t, filepath.Join(sinkDir, "secret.yaml"), `apiVersion: v1
kind: Secret
metadata:
  name: object-creds
  namespace: `+namespace+`
stringData:
  accessKeyID: AKIDTEST
  secretAccessKey: test-secret
`)

			s3Factories = 0
			exit := runCollectForTest(t, "--config", sinkDir, "--kubeconfig", kubeconfig)

			if exit != ExitFatalError {
				t.Errorf("exit = %d, want %d (ExitFatalError)", exit, ExitFatalError)
			}

			if len(results) != 1 {
				t.Fatalf("got %d context results, want 1", len(results))
			}

			res := results[0]
			if !errors.Is(res.Fatal, tc.want) {
				t.Fatalf("context fatal = %v, want it to wrap %v", res.Fatal, tc.want)
			}

			if !strings.HasPrefix(res.Fatal.Error(), "build sink backend: ") {
				t.Errorf("context fatal = %q, want the backend-build failure that follows collection", res.Fatal)
			}

			if s3Factories != 1 {
				t.Errorf("s3 factory called %d times, want 1 (collection finished, then the build failed)", s3Factories)
			}

			if res.Exported != 0 {
				t.Errorf("exported = %d, want 0", res.Exported)
			}
		})
	}
}

// runCollectForTest runs `collect` with args and returns its exit code; stderr is discarded.
func runCollectForTest(t *testing.T, args ...string) int {
	t.Helper()

	cmd, code := newCollectCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("collect %v: %v", args, err)
	}

	return *code
}

// writeCAParityConfigDir writes one ConfigMap profile and its target into a fresh directory.
func writeCAParityConfigDir(t *testing.T, namespace string) string {
	t.Helper()

	dir := t.TempDir()
	writeFileForTest(t, filepath.Join(dir, "profile.yaml"), `apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: configmaps
  namespace: `+namespace+`
spec:
  targetGVK:
    version: v1
    kind: ConfigMap
  attributes:
    - name: colour
      path: "$.data.colour"
`)
	writeFileForTest(t, filepath.Join(dir, "target.yaml"), `apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: configmap-target
  namespace: `+namespace+`
spec:
  profileRef: configmaps
  includedNamespaces:
    - `+namespace+`
`)

	return dir
}

func writeFileForTest(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// seedCAParityObjects creates the namespace and one ConfigMap for the profile to collect.
func seedCAParityObjects(t *testing.T, cfg *rest.Config, namespace string) {
	t.Helper()

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}

	ctx := context.Background()

	if _, err := kube.CoreV1().Namespaces().Create(ctx,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create namespace: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: namespace},
		Data:       map[string]string{"colour": "blue"},
	}
	if _, err := kube.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create configmap: %v", err)
	}
}

// writeEnvtestKubeconfigCLI writes a kubeconfig whose current context is the envtest server.
func writeEnvtestKubeconfigCLI(t *testing.T, cfg *rest.Config) string {
	t.Helper()

	apiCfg := clientcmdapi.NewConfig()
	apiCfg.Clusters["envtest"] = &clientcmdapi.Cluster{Server: cfg.Host, CertificateAuthorityData: cfg.CAData}
	apiCfg.AuthInfos["envtest"] = &clientcmdapi.AuthInfo{ClientCertificateData: cfg.CertData, ClientKeyData: cfg.KeyData}
	apiCfg.Contexts["envtest"] = &clientcmdapi.Context{Cluster: "envtest", AuthInfo: "envtest"}
	apiCfg.CurrentContext = "envtest"

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*apiCfg, path); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	return path
}

// isolateCLIAWSEnv points the AWS SDK's shared config at an empty file and clears the endpoint
// variables, so the host's AWS setup cannot change which check fires first.
func isolateCLIAWSEnv(t *testing.T) {
	t.Helper()

	empty := filepath.Join(t.TempDir(), "empty")
	writeFileForTest(t, empty, "")

	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "")
}
