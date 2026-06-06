// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// HelmReleasePathPrefix selects fields from a decoded helm.sh/v1 release Secret payload.
	HelmReleasePathPrefix = "helm:release."
)

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// helmReleaseFieldAliases map profile-friendly names to paths in the decoded release JSON.
var helmReleaseFieldAliases = map[string]string{
	"releaseName":  "name",
	"chartName":    "chart.metadata.name",
	"chartVersion": "chart.metadata.version",
	"appVersion":   "chart.metadata.appVersion",
	"revision":     "version",
	"status":       "info.status",
	"lastDeployed": "info.last_deployed",
	"namespace":    "namespace",
	"config":       "config",
}

// helmReleaseDeniedPaths block export of raw manifest blobs and chart file payloads.
var helmReleaseDeniedPaths = map[string]struct{}{
	"manifest":        {},
	"hooks":           {},
	"chart.templates": {},
	"chart.files":     {},
	"chart.values":    {},
	"chart.raw":       {},
}

// DecodeHelmReleaseSecret expands data.release from a helm.sh/v1 Secret into a release object map.
func DecodeHelmReleaseSecret(obj *unstructured.Unstructured) (map[string]any, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil object")
	}

	encoded, err := secretReleaseData(obj.Object)
	if err != nil {
		return nil, err
	}

	return decodeHelmReleasePayload(encoded)
}

func secretReleaseData(obj map[string]any) ([]byte, error) {
	data, found, err := unstructured.NestedMap(obj, "data")
	if err != nil {
		return nil, fmt.Errorf("read secret data: %w", err)
	}

	if !found || data == nil {
		return nil, fmt.Errorf("secret has no data")
	}

	raw, ok := data["release"]
	if !ok {
		return nil, fmt.Errorf("secret data.release not found")
	}

	switch v := raw.(type) {
	case string:
		decoded, decodeErr := base64.StdEncoding.DecodeString(v)
		if decodeErr != nil {
			return nil, fmt.Errorf("base64 decode data.release: %w", decodeErr)
		}

		return decoded, nil
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("secret data.release has unsupported type %T", raw)
	}
}

func decodeHelmReleasePayload(payload []byte) (map[string]any, error) {
	// Helm stores base64(gzip(JSON)); older releases may be uncompressed JSON.
	decoded, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		decoded = payload
	}

	if len(decoded) > 3 && bytes.Equal(decoded[0:3], magicGzip) {
		gr, gzipErr := gzip.NewReader(bytes.NewReader(decoded))
		if gzipErr != nil {
			return nil, fmt.Errorf("gzip decode release: %w", gzipErr)
		}

		decompressed, readErr := io.ReadAll(gr)
		_ = gr.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read gzip release: %w", readErr)
		}

		decoded = decompressed
	}

	var release map[string]any
	if unmarshalErr := json.Unmarshal(decoded, &release); unmarshalErr != nil {
		return nil, fmt.Errorf("json decode release: %w", unmarshalErr)
	}

	return release, nil
}

func extractHelmReleaseField(obj *unstructured.Unstructured, path string) (any, error) {
	field := strings.TrimPrefix(strings.TrimSpace(path), HelmReleasePathPrefix)
	if field == "" {
		return nil, fmt.Errorf("empty helm release field")
	}

	resolved := resolveHelmReleaseField(field)
	if err := validateHelmReleaseField(resolved); err != nil {
		return nil, err
	}

	release, err := DecodeHelmReleaseSecret(obj)
	if err != nil {
		return nil, err
	}

	val, found, err := nestedFieldFromRelease(release, resolved)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	return val, nil
}

func resolveHelmReleaseField(field string) string {
	if alias, ok := helmReleaseFieldAliases[field]; ok {
		return alias
	}

	return field
}

func validateHelmReleaseField(field string) error {
	if field == "" {
		return fmt.Errorf("empty helm release field")
	}

	if _, denied := helmReleaseDeniedPaths[field]; denied {
		return fmt.Errorf("helm release field %q is not exportable", field)
	}

	for denied := range helmReleaseDeniedPaths {
		if strings.HasPrefix(field, denied+".") {
			return fmt.Errorf("helm release field %q is not exportable", field)
		}
	}

	return nil
}

func nestedFieldFromRelease(release map[string]any, path string) (any, bool, error) {
	if path == "" {
		return nil, false, fmt.Errorf("empty path")
	}

	parts := strings.Split(path, ".")
	return unstructured.NestedFieldCopy(release, parts...)
}

// HelmReleasePathRequiresSecretOptIn reports whether a helm: path exposes Helm values/config.
func HelmReleasePathRequiresSecretOptIn(path string) bool {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, HelmReleasePathPrefix) {
		return false
	}

	field := resolveHelmReleaseField(strings.TrimPrefix(path, HelmReleasePathPrefix))

	return field == "config" || strings.HasPrefix(field, "config.")
}

// ValidateHelmReleaseAttributePath checks helm:release.<field> syntax and export policy.
func ValidateHelmReleaseAttributePath(path string) error {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, HelmReleasePathPrefix) {
		return fmt.Errorf("helm release paths must use %q prefix", HelmReleasePathPrefix)
	}

	field := strings.TrimPrefix(path, HelmReleasePathPrefix)
	if field == "" {
		return fmt.Errorf("empty helm release field")
	}

	return validateHelmReleaseField(resolveHelmReleaseField(field))
}
