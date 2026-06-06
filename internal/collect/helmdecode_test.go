// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func sampleHelmReleaseJSON() map[string]any {
	return map[string]any{
		"name":      "my-app",
		"namespace": "default",
		"version":   float64(2),
		"info": map[string]any{
			"status":         "deployed",
			"last_deployed":  "2024-01-15T10:30:00Z",
			"first_deployed": "2024-01-01T10:30:00Z",
		},
		"chart": map[string]any{
			"metadata": map[string]any{
				"name":       "nginx",
				"version":    "14.3.0",
				"appVersion": "2.1.0",
			},
		},
		"config": map[string]any{
			"replicaCount": float64(3),
		},
		"manifest": "apiVersion: v1\nkind: Service\n",
	}
}

func encodeHelmReleaseForSecret(release map[string]any) (string, error) {
	raw, err := json.Marshal(release)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}

	helmPayload := base64.StdEncoding.EncodeToString(buf.Bytes())
	return base64.StdEncoding.EncodeToString([]byte(helmPayload)), nil
}

func helmReleaseSecretObject(release map[string]any) (*unstructured.Unstructured, error) {
	encoded, err := encodeHelmReleaseForSecret(release)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"type":       "helm.sh/release.v1",
		"metadata": map[string]any{
			"name":      "sh.helm.release.v1.my-app.v2",
			"namespace": "default",
			"labels": map[string]any{
				"owner":  "helm",
				"name":   "my-app",
				"status": "deployed",
			},
		},
		"data": map[string]any{
			"release": encoded,
		},
	}}, nil
}

func TestDecodeHelmReleaseSecret(t *testing.T) {
	t.Parallel()

	obj, err := helmReleaseSecretObject(sampleHelmReleaseJSON())
	if err != nil {
		t.Fatalf("helmReleaseSecretObject: %v", err)
	}

	got, err := DecodeHelmReleaseSecret(obj)
	if err != nil {
		t.Fatalf("DecodeHelmReleaseSecret: %v", err)
	}

	if got["name"] != "my-app" {
		t.Fatalf("name = %v, want my-app", got["name"])
	}

	chart, ok := got["chart"].(map[string]any)
	if !ok {
		t.Fatal("chart is not an object")
	}

	metadata, ok := chart["metadata"].(map[string]any)
	if !ok {
		t.Fatal("chart.metadata is not an object")
	}

	if metadata["version"] != "14.3.0" {
		t.Fatalf("chart.metadata.version = %v, want 14.3.0", metadata["version"])
	}
}

func TestExtractHelmReleaseField(t *testing.T) {
	t.Parallel()

	obj, err := helmReleaseSecretObject(sampleHelmReleaseJSON())
	if err != nil {
		t.Fatalf("helmReleaseSecretObject: %v", err)
	}

	tests := []struct {
		path string
		want any
	}{
		{path: "helm:release.chartVersion", want: "14.3.0"},
		{path: "helm:release.appVersion", want: "2.1.0"},
		{path: "helm:release.releaseName", want: "my-app"},
		{path: "helm:release.revision", want: float64(2)},
		{path: "helm:release.status", want: "deployed"},
		{path: "helm:release.lastDeployed", want: "2024-01-15T10:30:00Z"},
		{path: "helm:release.chart.metadata.version", want: "14.3.0"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			got, extractErr := extractHelmReleaseField(obj, tt.path)
			if extractErr != nil {
				t.Fatalf("extractHelmReleaseField: %v", extractErr)
			}

			if got != tt.want {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestExtractHelmReleaseField_denied(t *testing.T) {
	t.Parallel()

	obj, err := helmReleaseSecretObject(sampleHelmReleaseJSON())
	if err != nil {
		t.Fatalf("helmReleaseSecretObject: %v", err)
	}

	_, err = extractHelmReleaseField(obj, "helm:release.manifest")
	if err == nil {
		t.Fatal("expected error for manifest path")
	}
}

func TestValidateHelmReleaseAttributePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    string
		wantErr bool
	}{
		{path: "helm:release.chartVersion", wantErr: false},
		{path: "helm:release.manifest", wantErr: true},
		{path: "helm:release.hooks", wantErr: true},
		{path: "helm:release.", wantErr: true},
		{path: "helm:other.field", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			err := ValidateHelmReleaseAttributePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHelmReleaseAttributePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHelmReleasePathRequiresSecretOptIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "helm:release.chartVersion", want: false},
		{path: "helm:release.config", want: true},
		{path: "helm:release.config.replicaCount", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			if got := HelmReleasePathRequiresSecretOptIn(tt.path); got != tt.want {
				t.Fatalf("HelmReleasePathRequiresSecretOptIn(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
