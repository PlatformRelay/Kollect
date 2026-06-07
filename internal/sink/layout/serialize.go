// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package layout projects an inventory snapshot into the readable file tree written by Git/GitLab
// snapshot sinks (ADR-0419). All functions are pure: given items and a resolved layout they return
// an ordered set of files, making golden tests per (mode, content, format) tuple straightforward.
package layout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	sigsyaml "sigs.k8s.io/yaml"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// ExtensionForFormat returns the canonical file extension (with leading dot) for a format.
func ExtensionForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case kollectdevv1alpha1.SerializationFormatYAML:
		return ".yaml"
	case kollectdevv1alpha1.SerializationFormatNDJSON:
		return ".ndjson"
	default:
		return ".json"
	}
}

// marshalValue serializes an arbitrary value (item, attributes map, manifest object) for a format.
func marshalValue(value any, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case kollectdevv1alpha1.SerializationFormatYAML:
		out, err := sigsyaml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %w", err)
		}

		return out, nil
	case kollectdevv1alpha1.SerializationFormatNDJSON:
		return marshalCompactJSONLine(value)
	default:
		out, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("json marshal: %w", err)
		}

		return append(out, '\n'), nil
	}
}

// marshalDocument serializes the whole inventory item list for document-mode files.
func marshalDocument(items []collect.Item, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case kollectdevv1alpha1.SerializationFormatNDJSON:
		var buf bytes.Buffer
		for i := range items {
			line, err := marshalCompactJSONLine(items[i])
			if err != nil {
				return nil, err
			}
			buf.Write(line)
		}

		return buf.Bytes(), nil
	default:
		return marshalValue(items, format)
	}
}

func marshalCompactJSONLine(value any) ([]byte, error) {
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	return append(out, '\n'), nil
}

// itemPayload returns the per-resource payload value for a content shape.
func itemPayload(item collect.Item, content, manifestKey string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(content)) {
	case kollectdevv1alpha1.LayoutContentAttributes:
		if item.Attributes == nil {
			return map[string]any{}, nil
		}

		return item.Attributes, nil
	case kollectdevv1alpha1.LayoutContentManifest:
		return manifestPayload(item, manifestKey)
	default:
		return item, nil
	}
}

// manifestPayload extracts the embedded Kubernetes object stored under manifestKey (ADR-0306).
func manifestPayload(item collect.Item, manifestKey string) (any, error) {
	key := strings.TrimSpace(manifestKey)
	if key == "" {
		key = kollectdevv1alpha1.DefaultExportAs
	}

	if item.Attributes == nil {
		return nil, fmt.Errorf("manifest content: item %q has no attributes", item.Name)
	}

	raw, ok := item.Attributes[key]
	if !ok {
		return nil, fmt.Errorf("manifest content: item %q missing attribute %q (Resource export required)", item.Name, key)
	}

	return raw, nil
}
