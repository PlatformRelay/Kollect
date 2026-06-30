// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package pathvalidate holds shared relative-path rules for Git and object-store export paths.
package pathvalidate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateRelativeObjectPath normalizes and rejects absolute paths, traversal, and null bytes.
func ValidateRelativeObjectPath(objectPath string) (string, error) {
	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		return "", nil
	}

	if strings.Contains(objectPath, "\x00") {
		return "", fmt.Errorf("object path contains null byte")
	}

	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(objectPath)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("object path must be relative")
	}

	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("object path must not contain '..' segments")
	}

	return clean, nil
}

// RejectTraversal returns an error when path is absolute or contains parent traversal.
func RejectTraversal(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	if strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return fmt.Errorf("path must be a relative path without '..' segments")
	}

	return nil
}

// InventoryFromObjectPath extracts the namespace and inventory name from a
// snapshot export object path of the form "inventory/<namespace>/<name>.json".
func InventoryFromObjectPath(objectPath string) (namespace, name string) {
	objectPath = strings.TrimPrefix(strings.TrimSpace(objectPath), "inventory/")
	parts := strings.Split(objectPath, "/")
	if len(parts) >= 2 {
		return parts[0], strings.TrimSuffix(parts[1], ".json")
	}

	if len(parts) == 1 && parts[0] != "" {
		return parts[0], ""
	}

	return "", ""
}
