// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package remotesecret

import (
	"encoding/base64"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestGenerateYAML(t *testing.T) {
	t.Parallel()

	out, err := GenerateYAML(Options{
		ClusterName: "spoke-a",
		Namespace:   "platform",
		APIServer:   "https://spoke-a.example:6443",
		Token:       "test-token",
		CAData:      "dGVzdC1jYQ==",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"name: " + kollectdevv1alpha1.RemoteSecretNamePrefix + "spoke-a",
		"namespace: platform",
		kollectdevv1alpha1.LabelMultiCluster + `: "true"`,
		kollectdevv1alpha1.AnnotationClusterName + ": spoke-a",
		"spoke-a:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	lines := strings.Split(out, "\n")
	var dataLine string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "spoke-a:") {
			dataLine = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "spoke-a:"))

			break
		}
	}

	if dataLine == "" {
		t.Fatal("missing data key")
	}

	decoded, err := base64.StdEncoding.DecodeString(dataLine)
	if err != nil {
		t.Fatal(err)
	}

	kubeconfig := string(decoded)
	for _, want := range []string{
		"server: https://spoke-a.example:6443",
		"token: test-token",
		"certificate-authority-data: dGVzdC1jYQ==",
	} {
		if !strings.Contains(kubeconfig, want) {
			t.Fatalf("kubeconfig missing %q:\n%s", want, kubeconfig)
		}
	}
}

func TestGenerateYAMLRequiresClusterName(t *testing.T) {
	t.Parallel()

	if _, err := GenerateYAML(Options{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateYAMLDefaults(t *testing.T) {
	t.Parallel()

	out, err := GenerateYAML(Options{ClusterName: "spoke-a"})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "namespace: platform") {
		t.Fatalf("output missing default namespace:\n%s", out)
	}

	lines := strings.Split(out, "\n")
	var dataLine string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "spoke-a:") {
			dataLine = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "spoke-a:"))

			break
		}
	}
	if dataLine == "" {
		t.Fatal("missing data key")
	}

	decoded, err := base64.StdEncoding.DecodeString(dataLine)
	if err != nil {
		t.Fatal(err)
	}

	kubeconfig := string(decoded)
	for _, want := range []string{
		"https://REPLACE_ME:6443",
		"REPLACE_WITH_SPOKE_SA_TOKEN",
		"REPLACE_WITH_BASE64_CA_DATA",
	} {
		if !strings.Contains(kubeconfig, want) {
			t.Fatalf("kubeconfig missing %q:\n%s", want, kubeconfig)
		}
	}
}
