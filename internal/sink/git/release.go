// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"fmt"
	"os"
	"path"

	billy "github.com/go-git/go-billy/v5"
)

// releaseCommitMessage is the subject of a deletion that only releases an ownership record
// (deletionPolicy Retain, ADR-0421/ADR-0422).
const releaseCommitMessage = "chore({cluster}/{namespace}/{name}): release inventory ownership record"

// releasePlan is what an inventory deletion removes from the checked-out branch, decided before
// anything is removed (ADR-0422).
type releasePlan struct {
	// candidates are the cleanup candidate paths matched with their part siblings (nil when the
	// files are kept).
	candidates []string
	// protected lists every path a record other than the deleting owner's claims, mapped to its
	// owner. A matched candidate in it is kept.
	protected map[string]string
	// exact are repository paths removed as they are: the deleting owner's recorded files that exist,
	// then its record.
	exact []string
}

// planRelease reads the ownership records a deletion must respect.
//
//   - ReleaseOnly (Retain): only the deleting owner's record is read, strictly, and it is the only
//     path removed. Other records are not read, so a damaged one cannot hold a Retain deletion.
//   - Otherwise every record is loaded strictly (loadPruneRecords); a candidate another owner
//     records is protected, and the deleting owner's recorded files and record are removed. Without
//     an owner (DeleteExport) every recorded path is protected.
//
// An absent record removes nothing of its own; that is a completed release, not an error.
func planRelease(fs billy.Filesystem, cfg Config, candidates []string) (releasePlan, error) {
	owner := cfg.PruneOwner
	if cfg.ReleaseOnly {
		if owner == "" {
			return releasePlan{}, invalidPrune("release without files needs an owner")
		}
		recordPath, found, err := ownRecord(fs, owner)
		if err != nil || !found {
			return releasePlan{}, err
		}

		return releasePlan{exact: []string{recordPath}}, nil
	}

	owners, records, _, err := loadPruneRecords(fs)
	if err != nil {
		return releasePlan{}, err
	}
	plan := releasePlan{candidates: candidates, protected: make(map[string]string, len(owners))}
	for p, o := range owners {
		if owner == "" || o != owner {
			plan.protected[p] = o
		}
	}
	if owner == "" {
		return plan, nil
	}
	record, found := records[owner]
	if !found {
		return plan, nil
	}
	for _, p := range record.Paths {
		exists, existsErr := regularFileExists(fs, p)
		if existsErr != nil {
			return releasePlan{}, existsErr
		}
		if exists {
			plan.exact = append(plan.exact, p)
		}
	}
	plan.exact = append(plan.exact, pruneRecordPath(owner))

	return plan, nil
}

// ownRecord validates the record at owner's record path: a regular file (no symlink anywhere on the
// path) that decodes strictly and names owner. A file there that names another owner is refused, so
// a release never removes a record that is not the deleting owner's.
func ownRecord(fs billy.Filesystem, owner string) (string, bool, error) {
	if len(owner) > 4096 {
		return "", false, invalidPrune("prune owner exceeds 4096 bytes")
	}
	recordPath := pruneRecordPath(owner)
	exists, err := regularFileExists(fs, recordPath)
	if err != nil || !exists {
		return recordPath, false, err
	}
	record, _, err := readPruneRecord(fs, path.Base(recordPath), maxPruneMetadataBytes)
	if err != nil {
		return "", false, err
	}
	if record.Owner != owner {
		return "", false, invalidPrune("ownership record %s names %s, not the deleting inventory",
			recordPath, describePruneOwner(record.Owner))
	}

	return recordPath, true, nil
}

// regularFileExists reports whether p exists, after checkPruneFile refused symlinks and non-regular
// files on its path.
func regularFileExists(fs billy.Filesystem, p string) (bool, error) {
	if err := checkPruneFile(fs, p); err != nil {
		return false, err
	}
	if _, err := fs.Lstat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("release lstat %q: %w", p, err)
	}

	return true, nil
}

// removeExact removes the plan's exact paths from fs. stage, when set, records each removal in the
// index; it reports false for a path the index never tracked, which is then not reported as removed.
func removeExact(fs billy.Filesystem, paths []string, stage func(string) (bool, error)) ([]string, error) {
	var removed []string
	for _, p := range paths {
		exists, err := regularFileExists(fs, p)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		if rmErr := fs.Remove(p); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return nil, fmt.Errorf("release remove %q: %w", p, rmErr)
		}
		if stage != nil {
			tracked, stageErr := stage(p)
			if stageErr != nil {
				return nil, stageErr
			}
			if !tracked {
				continue
			}
		}
		removed = append(removed, p)
	}

	return removed, nil
}

// appendNew appends the paths of more that are not in dst.
func appendNew(dst, more []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, p := range dst {
		seen[p] = struct{}{}
	}
	for _, p := range more {
		if _, dup := seen[p]; !dup {
			seen[p] = struct{}{}
			dst = append(dst, p)
		}
	}

	return dst
}
