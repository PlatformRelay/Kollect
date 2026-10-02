// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	billy "github.com/go-git/go-billy/v5"
)

// managedDirs returns the unique parent directories of the written paths (slash form), excluding
// the repository root. Pruning is scoped to these directories so a layout export removes stale
// resource files without clobbering unrelated trees written by other inventories.
func managedDirs(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	dirs := make([]string, 0, len(paths))
	for _, p := range paths {
		dir := path.Dir(p)
		if dir == "." || dir == "/" || dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}

	return dirs
}

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		set[p] = struct{}{}
	}

	return set
}

// removeBillyOrphans deletes files in managed directories that are not part of the new write set
// (go-git engine). Removed files are picked up by stageChanges' prune path as worktree deletions.
func removeBillyOrphans(fs billy.Filesystem, written []string) error {
	keep := pathSet(written)
	for _, dir := range managedDirs(written) {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("prune read dir %q: %w", dir, err)
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			p := path.Join(dir, e.Name())
			if _, ok := keep[p]; ok {
				continue
			}

			if err := fs.Remove(p); err != nil {
				return fmt.Errorf("prune remove %q: %w", p, err)
			}
		}
	}

	return nil
}

// removeDiskOrphans deletes files in managed directories that are not part of the new write set
// (CLI engine). Removed files are staged by the subsequent git add -A.
func removeDiskOrphans(workdir string, written []string) error {
	keep := pathSet(written)
	for _, dir := range managedDirs(written) {
		full := filepath.Join(workdir, filepath.FromSlash(dir))
		entries, err := os.ReadDir(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("prune read dir %q: %w", dir, err)
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			rel := path.Join(dir, e.Name())
			if _, ok := keep[rel]; ok {
				continue
			}

			if err := os.Remove(filepath.Join(full, e.Name())); err != nil {
				return fmt.Errorf("prune remove %q: %w", rel, err)
			}
		}
	}

	return nil
}
