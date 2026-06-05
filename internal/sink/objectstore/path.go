// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package objectstore holds shared helpers for Git/S3/GCS snapshot path layout (ADR-0401, ADR-0407).
package objectstore

import (
	"fmt"
	"regexp"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const (
	FormatJSON            = "json"
	FormatParquet         = "parquet"
	DefaultPathTemplate   = "inventory/{namespace}/{name}.json"
	DefaultExtension      = ".json"
)

var (
	hiveSegmentSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	allowedPlaceholders  = map[string]struct{}{
		"cluster":    {},
		"namespace":  {},
		"name":       {},
		"generation": {},
		"extension":  {},
	}
	placeholderPattern = regexp.MustCompile(`\{([a-z]+)\}`)
)

// PathVars carries values substituted into spec.pathTemplate (ADR-0407).
type PathVars struct {
	Cluster    string
	Namespace  string
	Name       string
	Generation int64
	Extension  string
}

// ObjectPath renders the export object path for a sink spec and inventory identity.
func ObjectPath(spec kollectdevv1alpha1.KollectSinkSpec, invNamespace, invName string, generation int64) string {
	if IsParquetFormat(spec) {
		return ParquetObjectPath(spec.Cluster, invNamespace, invName, generation)
	}

	template := strings.TrimSpace(spec.PathTemplate)
	if template == "" {
		template = DefaultPathTemplate
	}

	return RenderPathTemplate(template, PathVars{
		Cluster:    strings.TrimSpace(spec.Cluster),
		Namespace:  invNamespace,
		Name:       invName,
		Generation: generation,
		Extension:  DefaultExtension,
	})
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

// RenderPathTemplate substitutes supported placeholders in a path template.
func RenderPathTemplate(template string, vars PathVars) string {
	template = strings.TrimSpace(template)
	if template == "" {
		template = DefaultPathTemplate
	}

	ext := strings.TrimSpace(vars.Extension)
	if ext == "" {
		ext = DefaultExtension
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	cluster := strings.TrimSpace(vars.Cluster)
	if cluster == "" {
		cluster = "default"
	}

	replacer := strings.NewReplacer(
		"{cluster}", cluster,
		"{namespace}", vars.Namespace,
		"{name}", vars.Name,
		"{generation}", fmt.Sprintf("%d", vars.Generation),
		"{extension}", ext,
	)

	return replacer.Replace(template)
}

// ValidatePathTemplate checks spec.pathTemplate placeholders and shape.
func ValidatePathTemplate(template string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}

	if strings.HasPrefix(template, "/") || strings.Contains(template, "..") {
		return fmt.Errorf("pathTemplate must be a relative path without '..' segments")
	}

	if !strings.Contains(template, "{namespace}") || !strings.Contains(template, "{name}") {
		return fmt.Errorf("pathTemplate must include {namespace} and {name}")
	}

	for _, match := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		if _, ok := allowedPlaceholders[match[1]]; !ok {
			return fmt.Errorf("pathTemplate contains unsupported placeholder {%s}", match[1])
		}
	}

	return nil
}

func IsParquetFormat(spec kollectdevv1alpha1.KollectSinkSpec) bool {
	if spec.ObjectStore == nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(spec.ObjectStore.Format), FormatParquet)
}

// InventoryFromObjectPath extracts inventory namespace and name from a canonical or rendered path.
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

	namespace = parts[len(parts)-2]
	base := parts[len(parts)-1]
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		name = base[:dot]
	} else {
		name = base
	}

	return namespace, name
}

func sanitizeHiveSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	return hiveSegmentSanitizer.ReplaceAllString(value, "_")
}
