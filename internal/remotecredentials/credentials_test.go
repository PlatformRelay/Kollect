// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package remotecredentials

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
