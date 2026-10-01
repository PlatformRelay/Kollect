// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

// pathDepth counts slash-separated segments. "{cluster}/{namespace}/{kind}" is 3.
func pathDepth(p string) int {
	p = path.Clean(p)
	if p == "." || p == "/" {
		return 0
	}

	return strings.Count(p, "/") + 1
}

// kindSiblingDirs lists sibling directories of a managed directory when that
// directory is at least three segments deep. The default per-resource layout is
// {cluster}/{sourceNamespace}/{kind}/{sourceName}. A kind that this export no
// longer writes is not in managedDirs, so its last file would otherwise stay.
// Shallower trees (inventory/{namespace}/file) are not expanded: a neighboring
// prefix at that depth belongs to another inventory.
func kindSiblingDirs(managed []string, list func(dir string) ([]string, error)) ([]string, error) {
	seen := make(map[string]struct{}, len(managed))
	for _, dir := range managed {
		seen[dir] = struct{}{}
	}

	var extra []string
	visitedParents := make(map[string]struct{})
	for _, dir := range managed {
		if pathDepth(dir) < 3 {
			continue
		}

		parent := path.Dir(dir)
		if _, ok := visitedParents[parent]; ok {
			continue
		}
		visitedParents[parent] = struct{}{}

		children, err := list(parent)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("prune read dir %q: %w", parent, err)
		}
		for _, name := range children {
			child := path.Join(parent, name)
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			extra = append(extra, child)
		}
	}

	return extra, nil
}

func pruneDirs(written []string, list func(dir string) ([]string, error)) ([]string, error) {
	dirs := managedDirs(written)
	extra, err := kindSiblingDirs(dirs, list)
	if err != nil {
		return nil, err
	}

	return append(dirs, extra...), nil
}

// removeBillyOrphans deletes files in managed directories that are not part of the new write set
// (go-git engine). Removed files are picked up by stageChanges' prune path as worktree deletions.
func removeBillyOrphans(fs billy.Filesystem, written []string) error {
	keep := pathSet(written)
	dirs, err := pruneDirs(written, func(dir string) ([]string, error) {
		entries, readErr := fs.ReadDir(dir)
		if readErr != nil {
			return nil, readErr
		}

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}

		return names, nil
	})
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		entries, readErr := fs.ReadDir(dir)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}

			return fmt.Errorf("prune read dir %q: %w", dir, readErr)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			rel := path.Join(dir, entry.Name())
			if _, ok := keep[rel]; ok {
				continue
			}

			if err := fs.Remove(rel); err != nil {
				return fmt.Errorf("prune remove %q: %w", rel, err)
			}
		}
	}

	return nil
}

// removeDiskOrphans deletes files in managed directories that are not part of the new write set
// (CLI engine). Removed files are staged by the subsequent git add -A.
func removeDiskOrphans(workdir string, written []string) error {
	keep := pathSet(written)
	dirs, err := pruneDirs(written, func(dir string) ([]string, error) {
		full := filepath.Join(workdir, filepath.FromSlash(dir))
		entries, readErr := os.ReadDir(full)
		if readErr != nil {
			return nil, readErr
		}

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}

		return names, nil
	})
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		full := filepath.Join(workdir, filepath.FromSlash(dir))
		entries, readErr := os.ReadDir(full)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}

			return fmt.Errorf("prune read dir %q: %w", dir, readErr)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			rel := path.Join(dir, entry.Name())
			if _, ok := keep[rel]; ok {
				continue
			}

			if err := os.Remove(filepath.Join(full, entry.Name())); err != nil {
				return fmt.Errorf("prune remove %q: %w", rel, err)
			}
		}
	}

	return nil
}
