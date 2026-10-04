// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// These tests cover openspec/changes/cli-sink-credential-refs (CSC-1..CSC-6): what the sink
// backend receives when kollect-pipeline runs from a config directory.

const sinkNamespace = "kollect-system"

func credentialsCAPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kollect-pipeline-test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// indent prefixes every line of s, for embedding a PEM in a YAML block scalar.
func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}

	return strings.Join(lines, "\n")
}

func writeConfigDir(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// gitSinkYAML renders a git KollectSnapshotSink; extra is appended under spec.
func gitSinkYAML(extra string) string {
	return `apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: inventory-git
  namespace: ` + sinkNamespace + `
spec:
  type: git
  endpoint: https://git.example.internal/platform/inventory.git
  pathTemplate: clusters/{cluster}/{namespace}/{name}.yaml
  cluster: test
` + extra
}

func secretYAML(name string, stringData map[string]string) string {
	var b strings.Builder

	b.WriteString("apiVersion: v1\nkind: Secret\nmetadata:\n  name: " + name + "\n  namespace: " + sinkNamespace + "\n")
	b.WriteString("stringData:\n")

	for k, v := range stringData {
		if strings.Contains(v, "\n") {
			b.WriteString("  " + k + ": |\n" + indent(v, "    ") + "\n")

			continue
		}

		b.WriteString("  " + k + ": " + v + "\n")
	}

	return b.String()
}

// backendInput is what the CLI handed the git sink factory, plus the real git backend the
// factory built from it.
type backendInput struct {
	build   sink.BuildContext
	backend *git.Backend
}

// resolveThroughCLI runs the production order of kollect-pipeline collect from a config
// directory up to backend construction (cmd/cli/collect.go: LoadConfig, ResolveSink,
// ResolveSinkSecretData; runContext: cliBuildContext, withCLIBackend). It returns what the
// factory received, or the first error and whether the factory was called.
func resolveThroughCLI(t *testing.T, dir string) (backendInput, bool, error) {
	t.Helper()

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	spec, err := ResolveSink(loaded, "")
	if err != nil {
		t.Fatalf("ResolveSink: %v", err)
	}

	secretData, err := ResolveSinkSecretData(spec, loaded.Secrets)
	if err != nil {
		return backendInput{}, false, err
	}

	ctx := context.Background()

	build, err := cliBuildContext(ctx, spec, secretData, loaded.Secrets)
	if err != nil {
		return backendInput{}, false, err
	}

	var got backendInput

	called := false
	reg := sink.NewRegistry()
	delegate := sink.NewRegistry()
	reg.Register(git.TypeName, func(s kollectdevv1alpha1.KollectSinkSpec, bc sink.BuildContext) (sink.Backend, error) {
		called = true
		got.build = bc

		b, buildErr := delegate.NewBackend(s, bc)
		if buildErr != nil {
			return nil, buildErr
		}

		gb, ok := b.(*git.Backend)
		if !ok {
			t.Fatalf("git factory built %T", b)
		}

		got.backend = gb

		return b, nil
	})

	_, _, err = withCLIBackend(ctx, "ctx-a", reg, spec, build, func(sink.Backend) (int, []error) { return 0, nil })

	return got, called, err
}

func TestCLICredentials_gitAuthSecretRefOnly(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"sink.yaml":     gitSinkYAML("  git:\n    auth:\n      type: token\n      secretRef:\n        name: git-auth\n"),
		"git-auth.yaml": secretYAML("git-auth", map[string]string{"token": "auth-token"}),
	})

	got, _, err := resolveThroughCLI(t, dir)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if string(got.build.SecretData["token"]) != "auth-token" {
		t.Fatalf("backend received secret data %v, want the git.auth.secretRef token", keys(got.build.SecretData))
	}
}

func TestCLICredentials_gitAuthOverridesDefaultSecret(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"sink.yaml": gitSinkYAML("  secretRef:\n    name: default-creds\n" +
			"  git:\n    auth:\n      type: token\n      secretRef:\n        name: git-auth\n"),
		"default.yaml":  secretYAML("default-creds", map[string]string{"token": "default-token"}),
		"git-auth.yaml": secretYAML("git-auth", map[string]string{"token": "auth-token"}),
	})

	got, _, err := resolveThroughCLI(t, dir)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if tok := string(got.build.SecretData["token"]); tok != "auth-token" {
		t.Fatalf("backend received token %q, want the git.auth.secretRef one and never default-creds", tok)
	}
}

func TestCLICredentials_caSecretReachesGitBackend(t *testing.T) {
	ca := credentialsCAPEM(t)
	dir := writeConfigDir(t, map[string]string{
		"sink.yaml":    gitSinkYAML("  tls:\n    caSecretRef:\n      name: private-ca\n"),
		"private.yaml": secretYAML("private-ca", map[string]string{"ca.crt": string(ca)}),
	})

	got, _, err := resolveThroughCLI(t, dir)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !bytes.Equal(got.build.CAPEM, ca) {
		t.Fatal("the git factory did not receive the tls.caSecretRef CA")
	}

	cfg := got.backend.Config()
	if !bytes.Equal(cfg.CABundle, ca) || cfg.TLS.RootCAs == nil {
		t.Fatal("the git backend was built without the private CA; it would trust the system roots instead")
	}
}

func TestCLICredentials_missingReferencesFailBeforeTheBackend(t *testing.T) {
	cases := map[string]string{
		"missing-ca":       "  tls:\n    caSecretRef:\n      name: absent-ca\n",
		"missing-git-auth": "  git:\n    auth:\n      type: token\n      secretRef:\n        name: absent-auth\n",
	}

	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeConfigDir(t, map[string]string{"sink.yaml": gitSinkYAML(extra)})

			_, called, err := resolveThroughCLI(t, dir)
			if err == nil {
				t.Fatal("a reference to a Secret that is not in the config directory was accepted")
			}

			if !strings.Contains(err.Error(), "absent-") {
				t.Fatalf("error does not name the missing Secret: %v", err)
			}

			if called {
				t.Fatal("the backend was built despite the missing Secret")
			}
		})
	}
}

func TestCLICredentials_envPlaceholderThroughGitAuth(t *testing.T) {
	t.Setenv("KOLLECT_TEST_GIT_TOKEN", "from-the-environment")

	dir := writeConfigDir(t, map[string]string{
		"sink.yaml":     gitSinkYAML("  git:\n    auth:\n      type: token\n      secretRef:\n        name: git-auth\n"),
		"git-auth.yaml": secretYAML("git-auth", map[string]string{"token": "${env:KOLLECT_TEST_GIT_TOKEN}"}), //nolint:gosec // G101: test fixture, not a credential
	})

	got, _, err := resolveThroughCLI(t, dir)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if tok := string(got.build.SecretData["token"]); tok != "from-the-environment" {
		t.Fatalf("token = %q, want the environment value", tok)
	}
}

func TestWithCLIBackend_closesAfterFailedExport(t *testing.T) {
	t.Parallel()

	backend := &closeCountBackend{}
	reg := sink.NewRegistry()
	reg.Register("probe", func(kollectdevv1alpha1.KollectSinkSpec, sink.BuildContext) (sink.Backend, error) {
		return backend, nil
	})

	_, errs, err := withCLIBackend(context.Background(), "ctx-a", reg,
		kollectdevv1alpha1.KollectSinkSpec{Type: "probe"}, sink.BuildContext{},
		func(sink.Backend) (int, []error) { return 0, []error{errors.New("export failed")} })
	if err != nil || len(errs) != 1 {
		t.Fatalf("withCLIBackend = errs %v, err %v", errs, err)
	}

	if !backend.closed {
		t.Fatal("the backend was not closed after a failed export")
	}
}

// TestCLICredentials_matchOperatorResolver uses the operator's resolver as the oracle: for the
// same snapshot sink and Secrets, the CLI must give the backend the same data and CA, or fail
// when the operator fails.
func TestCLICredentials_matchOperatorResolver(t *testing.T) {
	ca := credentialsCAPEM(t)
	otherCA := credentialsCAPEM(t)

	secret := func(name string, data map[string]string) corev1.Secret {
		s := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace}, Data: map[string][]byte{}}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}

		return s
	}

	all := []corev1.Secret{
		secret("default-creds", map[string]string{"token": "default-token"}),
		secret("git-auth", map[string]string{"token": "auth-token", "known_hosts": "host key"}),
		secret("ca-crt", map[string]string{"ca.crt": string(ca)}),
		secret("tls-crt", map[string]string{"tls.crt": string(ca), "ca.crt": string(otherCA)}),
		secret("no-ca-key", map[string]string{"other": "x"}),
	}

	ref := func(name string) *kollectdevv1alpha1.SecretReference {
		return &kollectdevv1alpha1.SecretReference{Name: name}
	}

	gitAuth := func(name string) *kollectdevv1alpha1.GitSpec {
		return &kollectdevv1alpha1.GitSpec{Auth: &kollectdevv1alpha1.GitAuthSpec{Type: "token", SecretRef: ref(name)}}
	}

	cases := map[string]kollectdevv1alpha1.KollectSinkSpec{
		"none":                 {Type: "git"},
		"secretRef":            {Type: "git", SecretRef: ref("default-creds")},
		"gitAuth":              {Type: "git", Git: gitAuth("git-auth")},
		"both":                 {Type: "git", SecretRef: ref("default-creds"), Git: gitAuth("git-auth")},
		"both, default absent": {Type: "git", SecretRef: ref("absent"), Git: gitAuth("git-auth")},
		"gitAuth absent":       {Type: "git", Git: gitAuth("absent")},
		"gitlab ignores gitAuth": {
			Type: "gitlab", SecretRef: ref("default-creds"), Git: gitAuth("git-auth"),
		},
		"s3 secretRef":          {Type: "s3", SecretRef: ref("default-creds")},
		"caSecret ca.crt":       {Type: "git", TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("ca-crt")}},
		"caSecret tls.crt wins": {Type: "git", TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("tls-crt")}},
		"caSecret no key":       {Type: "git", TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("no-ca-key")}},
		"caSecret absent":       {Type: "git", TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("absent")}},
		"inline bundle wins": {Type: "git", TLS: &kollectdevv1alpha1.TLSSpec{
			CABundle: otherCA, CASecretRef: ref("absent"),
		}},
		"s3 caSecret": {Type: "s3", SecretRef: ref("default-creds"),
			TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("ca-crt")}},
	}

	objs := make([]corev1.Secret, len(all))
	copy(objs, all)

	builder := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme)
	for i := range objs {
		builder = builder.WithObjects(&objs[i])
	}

	operatorClient := builder.Build()

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			want, wantErr := sink.BuildContextFromSpec(context.Background(), operatorClient, spec, sinkNamespace)

			var got sink.BuildContext

			data, gotErr := ResolveSinkSecretData(spec, all)
			if gotErr == nil {
				got, gotErr = cliBuildContext(context.Background(), spec, data, all)
			}

			if (wantErr != nil) != (gotErr != nil) {
				t.Fatalf("operator error %v, CLI error %v: one fails, the other does not", wantErr, gotErr)
			}

			if wantErr != nil {
				return
			}

			if !sameData(want.SecretData, got.SecretData) {
				t.Errorf("secret data: operator %v, CLI %v", keys(want.SecretData), keys(got.SecretData))
			}

			if !bytes.Equal(want.CAPEM, got.CAPEM) {
				t.Errorf("CA: operator %d bytes, CLI %d bytes", len(want.CAPEM), len(got.CAPEM))
			}
		})
	}
}

// sameData compares secret data, treating nil and empty as equal.
func sameData(a, b map[string][]byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	return reflect.DeepEqual(a, b)
}

// keys lists the keys of m with the length of each value; never the values, which are secrets.
func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s(%d bytes)", k, len(v)))
	}

	sort.Strings(out)

	return out
}
