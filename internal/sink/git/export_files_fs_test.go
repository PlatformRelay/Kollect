// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	billy "github.com/go-git/go-billy/v5"
)

// ExportFilesToFilesystemForTest is the in-memory twin of Backend.ExportFiles for the external
// owned-prune fuzz target (package git_test). It merges opts into the backend config through the same
// filesConfig as ExportFiles, runs the same file-set validation as ExportFilesWithBranch, and then the real
// write + owned-prune engine (writeBillyExportFiles: prepareOwnedPrune, writes, plan.apply) against
// fs. Only clone, commit and push are skipped, so a fuzz step costs microseconds instead of a git
// round trip. Test-only: compiled into this package's test binary, never into the product.
func (b *Backend) ExportFilesToFilesystemForTest(fs billy.Filesystem, files []FileEntry, opts ExportFilesOptions) error {
	cfg := b.filesConfig(opts).withDefaults()

	_, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		return err
	}
	_, err = writeBillyExportFiles(fs, cfg, validated)

	return err
}

// ReleaseExportToFilesystemForTest is the in-memory twin of Backend.ReleaseExport for the owned-prune
// fuzz target: the same config (ReleaseConfig, deletionConfig), path validation (validateDeletePaths)
// and release engine (planRelease, candidate and exact removal) against fs, without clone, commit or
// push. Test-only.
func (b *Backend) ReleaseExportToFilesystemForTest(fs billy.Filesystem, paths []string, opts ReleaseOptions) ([]string, error) {
	cfg := deletionConfig(ReleaseConfig(b.cfg, opts))

	_, validated, err := validateDeletePaths(cfg, paths, nil)
	if err != nil {
		return nil, err
	}
	if len(validated) == 0 && cfg.PruneOwner == "" {
		return nil, nil
	}

	return releaseFS(fs, cfg, validated, nil)
}
