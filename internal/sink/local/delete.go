// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package local

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/platformrelay/kollect/internal/sink/objectstore"
)

// DeleteExport removes the inventory's exported file and its deterministic
// .part-NNNN-of-NNNN siblings from the output directory (K-28, C-2a) and returns
// the deleted paths relative to that directory. Missing files are not errors;
// every candidate path is resolved through safeJoin so cleanup can never delete
// outside the output directory.
func (b *Backend) DeleteExport(_ context.Context, paths []string) ([]string, error) {
	var deleted []string

	for _, objectPath := range paths {
		got, err := b.removeMatching(objectstore.CleanupMatchers(objectPath))
		if err != nil {
			return deleted, err
		}
		deleted = append(deleted, got...)
	}

	return deleted, nil
}

func (b *Backend) removeMatching(matchers []objectstore.KeyMatcher) ([]string, error) {
	var deleted []string

	for _, m := range matchers {
		got, err := b.removeMatchingDir(m)
		if err != nil {
			return deleted, err
		}
		deleted = append(deleted, got...)
	}

	return deleted, nil
}

func (b *Backend) removeMatchingDir(m objectstore.KeyMatcher) ([]string, error) {
	var deleted []string

	listDir := m.ListDir()

	baseDir, err := b.safeJoin(listDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("local sink cleanup read dir %q: %w", listDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		full := objectstore.JoinDir(listDir, entry.Name())
		if !m.Matches(full) {
			continue
		}

		fullPath, err := b.safeJoin(full)
		if err != nil {
			return deleted, err
		}

		if rmErr := os.Remove(fullPath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return deleted, fmt.Errorf("local sink cleanup remove %q: %w", full, rmErr)
		}

		deleted = append(deleted, full)
	}

	return deleted, nil
}
