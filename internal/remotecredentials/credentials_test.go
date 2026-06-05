// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package remotecredentials

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/konih/kollect/internal/remotesecret"
)

type stubChecker struct {
	called bool
}

func (s *stubChecker) Check(_ context.Context, _ []byte) error {
	s.called = true

	return nil
}

func TestExtractAndValidateKubeconfig(t *testing.T) {
	t.Parallel()

	yaml, err := remotesecret.GenerateYAML(remotesecret.Options{
		ClusterName: "spoke-a",
		Namespace:   "platform",
		APIServer:   "https://spoke.example:6443",
		Token:       "test-token",
		CAData:      base64.StdEncoding.EncodeToString([]byte("ca")),
	})
	if err != nil {
		t.Fatal(err)
	}

	dataLine := strings.Split(yaml, "  spoke-a: ")[1]
	dataLine = strings.TrimSpace(strings.Split(dataLine, "\n")[0])
	raw, err := base64.StdEncoding.DecodeString(dataLine)
	if err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "kollect-remote-secret-spoke-a"},
		Data:       map[string][]byte{"spoke-a": raw},
	}

	extracted, err := ExtractKubeconfig(secret, "spoke-a")
	if err != nil {
		t.Fatal(err)
	}

	if err := ValidateFragment(extracted); err != nil {
		t.Fatalf("ValidateFragment: %v", err)
	}

	checker := &stubChecker{}
	if err := VerifySecret(context.Background(), secret, "spoke-a", checker); err != nil {
		t.Fatalf("VerifySecret: %v", err)
	}
	if !checker.called {
		t.Fatal("expected API checker to run")
	}

	if err := VerifySecret(context.Background(), secret, "spoke-a", nil); err != nil {
		t.Fatalf("VerifySecret without checker: %v", err)
	}
}

func TestExtractKubeconfigMissingKey(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "empty"},
		Data:       map[string][]byte{"other": []byte("x")},
	}

	if _, err := ExtractKubeconfig(secret, "spoke-a"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestExtractKubeconfigValidationErrors(t *testing.T) {
	t.Parallel()

	if _, err := ExtractKubeconfig(nil, "spoke-a"); err == nil {
		t.Fatal("expected error for nil secret")
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "empty"}}
	if _, err := ExtractKubeconfig(secret, ""); err == nil {
		t.Fatal("expected error for empty cluster name")
	}

	secret.Data = map[string][]byte{"spoke-a": {}}
	if _, err := ExtractKubeconfig(secret, "spoke-a"); err == nil {
		t.Fatal("expected error for empty kubeconfig bytes")
	}
}

func TestSecretGetErrorClass(t *testing.T) {
	t.Parallel()

	if SecretGetErrorClass(nil) != "" {
		t.Fatal("nil err should be empty class")
	}

	nf := apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "x")
	if SecretGetErrorClass(nf) != "terminal" {
		t.Fatalf("not found class = %q", SecretGetErrorClass(nf))
	}

	if SecretGetErrorClass(context.Canceled) != "transient" {
		t.Fatalf("generic class = %q", SecretGetErrorClass(context.Canceled))
	}
}

func TestDefaultAPICheckerRejectsInvalidKubeconfig(t *testing.T) {
	t.Parallel()

	checker := DefaultAPIChecker{}
	if err := checker.Check(context.Background(), []byte("not-a-kubeconfig")); err == nil {
		t.Fatal("expected error for invalid kubeconfig")
	}
}

func TestValidateFragmentErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
	}{
		{name: "invalid yaml", yaml: "{"},
		{name: "no clusters", yaml: `apiVersion: v1
kind: Config
users:
- name: u
  user:
    token: tok`},
		{name: "no users", yaml: `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example:6443`},
		{name: "missing server", yaml: `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster: {}
users:
- name: u
  user:
    token: tok`},
		{name: "missing credentials", yaml: `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example:6443
users:
- name: u
  user: {}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateFragment([]byte(tc.yaml)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestExtractKubeconfigNilData(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "empty"}}
	if _, err := ExtractKubeconfig(secret, "spoke-a"); err == nil {
		t.Fatal("expected error for nil secret data")
	}
}

func TestDefaultAPICheckerProbeUnreachableAPIServer(t *testing.T) {
	t.Parallel()

	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:1
    insecure-skip-tls-verify: true
users:
- name: u
  user:
    token: tok
contexts:
- name: x
  context:
    cluster: c
    user: u
current-context: x
`)

	if err := (DefaultAPIChecker{}).Check(context.Background(), kubeconfig); err == nil {
		t.Fatal("expected probe error for unreachable API server")
	}
}
