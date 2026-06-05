// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package parquet encodes inventory snapshots to Parquet (ADR-0401 hybrid schema, Q11).
package parquet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/konih/kollect/internal/collect"
)

const contentType = "application/vnd.apache.parquet"

var DefaultHotAttributes = []string{"image", "version"}

type EncodeOptions struct {
	Cluster            string
	InventoryNamespace string
	InventoryName      string
	HotAttributes      []string
	ExportedAt         time.Time
}

func ContentType() string {
	return contentType
}

func EncodeItems(items []collect.Item, opts EncodeOptions) ([]byte, error) {
	hotAttrs := normalizeHotAttributes(opts.HotAttributes)
	schema := buildSchema(hotAttrs)

	exportedAt := opts.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}

	buf := new(bytes.Buffer)
	writer := parquetgo.NewWriter(buf, schema)

	for _, item := range items {
		attributesJSON, err := json.Marshal(item.Attributes)
		if err != nil {
			return nil, fmt.Errorf("parquet encode: marshal attributes: %w", err)
		}

		row := map[string]any{
			"cluster":             strings.TrimSpace(opts.Cluster),
			"inventory_namespace": strings.TrimSpace(opts.InventoryNamespace),
			"inventory_name":      strings.TrimSpace(opts.InventoryName),
			"target_namespace":    item.TargetNamespace,
			"target_name":         item.TargetName,
			"namespace":           item.Namespace,
			"name":                item.Name,
			"uid":                 item.UID,
			"api_version":         item.Version,
			"kind":                item.Kind,
			"exported_at":         exportedAt,
			"attributes":          attributesJSON,
		}

		if item.Group != "" {
			row["group"] = item.Group
		}

		for _, attr := range hotAttrs {
			if v := attributeValue(item.Attributes, attr); v != nil {
				row[columnName(attr)] = *v
			}
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("parquet encode: write row: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("parquet encode: close writer: %w", err)
	}

	return buf.Bytes(), nil
}

func normalizeHotAttributes(attrs []string) []string {
	if len(attrs) == 0 {
		attrs = append([]string(nil), DefaultHotAttributes...)
	}

	seen := make(map[string]struct{}, len(attrs))
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		attr = strings.TrimSpace(attr)
		if attr == "" {
			continue
		}

		key := strings.ToLower(attr)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, attr)
	}

	sort.Strings(out)

	return out
}

func buildSchema(hotAttrs []string) *parquetgo.Schema {
	group := parquetgo.Group{
		"cluster":             parquetgo.String(),
		"inventory_namespace": parquetgo.String(),
		"inventory_name":      parquetgo.String(),
		"target_namespace":    parquetgo.String(),
		"target_name":         parquetgo.String(),
		"namespace":           parquetgo.String(),
		"name":                parquetgo.String(),
		"uid":                 parquetgo.String(),
		"group":               parquetgo.Optional(parquetgo.String()),
		"api_version":         parquetgo.String(),
		"kind":                parquetgo.String(),
		"exported_at":         parquetgo.Timestamp(parquetgo.Microsecond),
		"attributes":          parquetgo.Leaf(parquetgo.ByteArrayType),
	}

	for _, attr := range hotAttrs {
		group[columnName(attr)] = parquetgo.Optional(parquetgo.String())
	}

	return parquetgo.NewSchema("inventory", group)
}

func columnName(attr string) string {
	return "attr_" + sanitizeColumn(attr)
}

func sanitizeColumn(attr string) string {
	attr = strings.TrimSpace(strings.ToLower(attr))
	if attr == "" {
		return "unknown"
	}

	var b strings.Builder
	for _, r := range attr {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune('_')
		}
	}

	if b.Len() == 0 {
		return "unknown"
	}

	return b.String()
}

func attributeValue(attrs map[string]any, key string) *string {
	if len(attrs) == 0 {
		return nil
	}

	value, ok := attrs[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}

		out := typed

		return &out
	case bool:
		out := fmt.Sprintf("%t", typed)

		return &out
	case float64:
		out := fmt.Sprintf("%v", typed)

		return &out
	default:
		raw, err := json.Marshal(typed)
		if err != nil || len(raw) == 0 || string(raw) == "null" {
			return nil
		}

		out := string(raw)

		return &out
	}
}
