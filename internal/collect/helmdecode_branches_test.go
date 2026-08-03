// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// helmSecretWithRelease builds a minimal helm-style Secret whose data.release
// holds an arbitrary (possibly malformed) value.
func helmSecretWithRelease(release any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "sh.helm.release.v1.x.v1", "namespace": "default"},
		"data":       map[string]any{"release": release},
	}}
}

func TestDecodeHelmReleaseSecret_structuralErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		wantMsg string
	}{
		{
			name:    "nil object",
			obj:     nil,
			wantMsg: "nil object",
		},
		{
			name: "data missing",
			obj: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Secret",
			}},
			wantMsg: "secret has no data",
		},
		{
			name: "data not a map",
			obj: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Secret", "data": "not-a-map",
			}},
			wantMsg: "read secret data",
		},
		{
			name:    "data.release missing",
			obj:     &unstructured.Unstructured{Object: map[string]any{"data": map[string]any{"other": "x"}}},
			wantMsg: "secret data.release not found",
		},
		{
			name:    "data.release invalid base64",
			obj:     helmSecretWithRelease("!!!not-base64!!!"),
			wantMsg: "base64 decode data.release",
		},
		{
			name:    "data.release unsupported type",
			obj:     helmSecretWithRelease(int64(42)),
			wantMsg: "unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeHelmReleaseSecret(tt.obj)
			if err == nil {
				t.Fatalf("DecodeHelmReleaseSecret() = %v, want error containing %q", got, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// TestDecodeHelmReleaseSecret_uncompressedJSON locks the legacy fallback: older
// Helm releases stored plain JSON (single base64, no gzip); the failed inner
// base64 decode must fall back to the raw payload instead of erroring.
func TestDecodeHelmReleaseSecret_uncompressedJSON(t *testing.T) {
	t.Parallel()

	rawJSON := `{"name":"legacy-app","version":7}`
	obj := helmSecretWithRelease(base64.StdEncoding.EncodeToString([]byte(rawJSON)))

	got, err := DecodeHelmReleaseSecret(obj)
	if err != nil {
		t.Fatalf("DecodeHelmReleaseSecret() error = %v, want legacy uncompressed JSON to decode", err)
	}
	if got["name"] != "legacy-app" {
		t.Fatalf("name = %v, want legacy-app", got["name"])
	}
	if got["version"] != float64(7) {
		t.Fatalf("version = %v, want 7", got["version"])
	}
}

// encodeRawHelmPayload wraps arbitrary inner payload bytes the way secretReleaseData
// hands them to decodeHelmReleasePayload: base64(base64(payload)) in data.release.
func encodeRawHelmPayload(payload []byte) string {
	inner := base64.StdEncoding.EncodeToString(payload)

	return base64.StdEncoding.EncodeToString([]byte(inner))
}

// TestDecodeHelmReleaseSecret_malformedPayloads is the EDGE lock: every malformed
// helm-release payload shape must produce a wrapped decode error — never a panic.
func TestDecodeHelmReleaseSecret_malformedPayloads(t *testing.T) {
	t.Parallel()

	gzipped := func(raw []byte) []byte {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(raw); err != nil {
			t.Fatalf("gzip write: %v", err)
		}
		if err := gw.Close(); err != nil {
			t.Fatalf("gzip close: %v", err)
		}

		return buf.Bytes()
	}

	corruptHeader := append([]byte{}, magicGzip...)
	corruptHeader = append(corruptHeader, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)

	validGzip := gzipped([]byte(`{"name":"app","chart":{"metadata":{"name":"c"}}}`))
	truncated := validGzip[:len(validGzip)-6]

	tests := []struct {
		name    string
		payload []byte
		wantMsg string
	}{
		{name: "corrupt gzip header", payload: corruptHeader, wantMsg: "gzip decode release"},
		{name: "truncated gzip stream", payload: truncated, wantMsg: "read gzip release"},
		{name: "gzip of non-JSON", payload: gzipped([]byte("not json at all")), wantMsg: "json decode release"},
		{name: "plain non-JSON non-gzip", payload: []byte("garbage"), wantMsg: "json decode release"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := helmSecretWithRelease(encodeRawHelmPayload(tt.payload))

			got, err := DecodeHelmReleaseSecret(obj)
			if err == nil {
				t.Fatalf("DecodeHelmReleaseSecret() = %v, want wrapped error containing %q", got, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestExtractHelmReleaseField_branches(t *testing.T) {
	t.Parallel()

	obj, err := helmReleaseSecretObject(sampleHelmReleaseJSON())
	if err != nil {
		t.Fatalf("helmReleaseSecretObject: %v", err)
	}

	t.Run("empty field errors", func(t *testing.T) {
		t.Parallel()

		if _, extractErr := extractHelmReleaseField(obj, "helm:release."); extractErr == nil {
			t.Fatal("expected error for empty helm release field")
		}
	})

	t.Run("decode error propagates", func(t *testing.T) {
		t.Parallel()

		malformed := helmSecretWithRelease(encodeRawHelmPayload([]byte("garbage")))
		_, extractErr := extractHelmReleaseField(malformed, "helm:release.releaseName")
		if extractErr == nil || !strings.Contains(extractErr.Error(), "json decode release") {
			t.Fatalf("error = %v, want wrapped json decode error", extractErr)
		}
	})

	t.Run("traversal through non-map errors", func(t *testing.T) {
		t.Parallel()

		_, extractErr := extractHelmReleaseField(obj, "helm:release.name.sub")
		if extractErr == nil {
			t.Fatal("expected error when traversing through a scalar field")
		}
	})

	t.Run("missing field returns nil without error", func(t *testing.T) {
		t.Parallel()

		val, extractErr := extractHelmReleaseField(obj, "helm:release.doesNotExist")
		if extractErr != nil {
			t.Fatalf("extractHelmReleaseField error = %v, want nil for absent field", extractErr)
		}
		if val != nil {
			t.Fatalf("val = %v, want nil for absent field", val)
		}
	})
}

func TestValidateHelmReleaseField_edge(t *testing.T) {
	t.Parallel()

	if err := validateHelmReleaseField(""); err == nil {
		t.Fatal("validateHelmReleaseField(\"\") expected error")
	}

	if err := ValidateHelmReleaseAttributePath("helm:release.chart.templates.foo"); err == nil {
		t.Fatal("expected denied-prefix error for chart.templates.foo")
	}
}

func TestNestedFieldFromRelease_emptyPath(t *testing.T) {
	t.Parallel()

	_, _, err := nestedFieldFromRelease(map[string]any{"a": "b"}, "")
	if err == nil {
		t.Fatal("nestedFieldFromRelease(\"\") expected error")
	}
}

func TestHelmReleasePathRequiresSecretOptIn_nonHelmPath(t *testing.T) {
	t.Parallel()

	if HelmReleasePathRequiresSecretOptIn("spec.values") {
		t.Fatal("non-helm path must never require secret opt-in")
	}
}
