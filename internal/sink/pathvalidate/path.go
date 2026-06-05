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
