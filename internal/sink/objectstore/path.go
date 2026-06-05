// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package objectstore holds shared helpers for S3/GCS snapshot sinks (ADR-0401, ADR-0407).
package objectstore

import (
	"fmt"
	"regexp"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const (
	FormatJSON    = "json"
	FormatParquet = "parquet"
)

var hiveSegmentSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func DefaultObjectPath(invNamespace, invName string) string {
	return fmt.Sprintf("inventory/%s/%s.json", invNamespace, invName)
}

func ParquetObjectPath(cluster, invNamespace, invName string, generation int64) string {
	cluster = sanitizeHiveSegment(cluster)
	if cluster == "" {
		cluster = "default"
	}

	return fmt.Sprintf(
		"inventory/cluster=%s/ns=%s/name=%s/generation=%d.parquet",
		cluster,
		sanitizeHiveSegment(invNamespace),
		sanitizeHiveSegment(invName),
		generation,
	)
}

func ObjectPath(spec kollectdevv1alpha1.KollectSinkSpec, invNamespace, invName string, generation int64) string {
	if IsParquetFormat(spec) {
		return ParquetObjectPath(spec.Cluster, invNamespace, invName, generation)
	}

	return DefaultObjectPath(invNamespace, invName)
}

func IsParquetFormat(spec kollectdevv1alpha1.KollectSinkSpec) bool {
	if spec.ObjectStore == nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(spec.ObjectStore.Format), FormatParquet)
}

func InventoryFromObjectPath(objectPath string) (namespace, name string) {
	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		return "", ""
	}

	if strings.Contains(objectPath, "/ns=") {
		for _, segment := range strings.Split(objectPath, "/") {
			if strings.HasPrefix(segment, "ns=") {
				namespace = strings.TrimPrefix(segment, "ns=")
			}

			if strings.HasPrefix(segment, "name=") {
				name = strings.TrimPrefix(segment, "name=")
			}
		}

		return namespace, name
	}

	objectPath = strings.TrimPrefix(objectPath, "inventory/")
	parts := strings.Split(objectPath, "/")
	if len(parts) < 2 {
		return "", ""
	}

	return parts[0], strings.TrimSuffix(parts[len(parts)-1], ".json")
}

func sanitizeHiveSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	return hiveSegmentSanitizer.ReplaceAllString(value, "_")
}
