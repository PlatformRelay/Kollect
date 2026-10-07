// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"fmt"
	"io"
	"path"
	"sort"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// checkMergeTargetClaims refuses, before anything is written, an export to a branch other than the
// one it was cloned from (a merge-request feature branch) whose claims would collide with the merge
// target. prepareOwnedPrune checks the checked-out feature branch, which in a warm mirror is this
// inventory's own branch as it was when it was last pushed; another inventory's claim merged into the
// target since then is not on it. Merging the feature branch would then leave two records claiming
// one path, and every export to the target would fail. targetRef is the freshly fetched target tip;
// a target without it (an empty remote) has no records.
func checkMergeTargetClaims(repo *git.Repository, targetRef plumbing.ReferenceName, targetBranch string, cfg Config, written []string) error {
	if cfg.PruneOwner == "" {
		return nil
	}
	ref, err := repo.Reference(targetRef, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("merge target %q: resolve %s: %w", targetBranch, targetRef, err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("merge target %q: load commit: %w", targetBranch, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("merge target %q: load tree: %w", targetBranch, err)
	}
	fs, err := pruneMetadataFromTree(tree)
	if err != nil {
		return fmt.Errorf("merge target %q: %w", targetBranch, err)
	}
	owners, _, _, err := loadPruneRecords(fs)
	if err != nil {
		return fmt.Errorf("merge target %q: %w", targetBranch, err)
	}

	// The paths this export writes or records, and the set manifest a later part will write.
	paths := append(append([]string(nil), pruneKeepSet(cfg, written)...), cfg.PruneClaimPaths...)
	sort.Strings(paths)
	for _, p := range paths {
		owner, ok := owners[p]
		if !ok || owner == cfg.PruneOwner {
			continue
		}

		return invalidPrune("prune path %q belongs to another inventory on merge target branch %q: %s, ownership record %s",
			p, targetBranch, describePruneOwner(owner), pruneRecordPath(owner))
	}

	return nil
}

// pruneMetadataFromTree copies the ownership-record directory of a commit tree into memory, keeping
// whatever makes loadPruneRecords reject it (a file or symlink where the directory belongs, a symlink
// or directory among the records) and bounding entries and bytes before reading any blob.
func pruneMetadataFromTree(tree *object.Tree) (billy.Filesystem, error) {
	fs := memfs.New()
	entry, err := tree.FindEntry(pruneMetadataDir)
	if errors.Is(err, object.ErrEntryNotFound) || errors.Is(err, object.ErrDirectoryNotFound) {
		return fs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prune metadata lookup: %w", err)
	}
	if entry.Mode != filemode.Dir {
		return fs, copyTreeEntry(fs, tree, entry, pruneMetadataDir, nil)
	}
	dir, err := tree.Tree(pruneMetadataDir)
	if err != nil {
		return nil, fmt.Errorf("prune metadata tree: %w", err)
	}
	if len(dir.Entries) > maxPruneOwners {
		return nil, invalidPrune("prune metadata exceeds %d owners", maxPruneOwners)
	}
	if err := fs.MkdirAll(pruneMetadataDir, 0o750); err != nil {
		return nil, fmt.Errorf("prune metadata mkdir: %w", err)
	}
	var total int64
	for i := range dir.Entries {
		if err := copyTreeEntry(fs, dir, &dir.Entries[i], path.Join(pruneMetadataDir, dir.Entries[i].Name), &total); err != nil {
			return nil, err
		}
	}

	return fs, nil
}

// copyTreeEntry writes one tree entry to fs at name: a regular file with its content (counted against
// total when non-nil), a symlink as a symlink, anything else as a directory.
func copyTreeEntry(fs billy.Filesystem, tree *object.Tree, entry *object.TreeEntry, name string, total *int64) error {
	switch entry.Mode {
	case filemode.Regular, filemode.Executable:
		file, err := tree.TreeEntryFile(entry)
		if err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
		if total != nil {
			*total += file.Size
			if *total > maxPruneMetadataBytes {
				return invalidPrune("prune metadata exceeds %d bytes", maxPruneMetadataBytes)
			}
		}
		reader, err := file.Reader()
		if err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, maxPruneMetadataBytes+1))
		if err := errors.Join(readErr, reader.Close()); err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
		if err := util.WriteFile(fs, name, data, 0o600); err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
	case filemode.Symlink:
		if err := fs.Symlink("target", name); err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
	default:
		if err := fs.MkdirAll(name, 0o750); err != nil {
			return fmt.Errorf("prune metadata %q: %w", name, err)
		}
	}

	return nil
}

// checkCLIMergeTargetClaims is checkMergeTargetClaims for the CLI machinery's workdir, where the target
// tip prepareCLIWorkdir fetched is the remote-tracking ref origin/<cloneBranch>. It runs in every
// branch mode: with push branch == clone branch a warm mirror checks out its stale local branch, so
// a claim pushed meanwhile is only visible on origin/<cloneBranch>, and the non-fast-forward
// pull --rebase that follows would otherwise merge both records in.
func checkCLIMergeTargetClaims(workdir, cloneBranch string, cfg Config, written []string) error {
	if cfg.PruneOwner == "" {
		return nil
	}
	repo, err := git.PlainOpen(workdir)
	if err != nil {
		return fmt.Errorf("merge target %q: open workdir: %w", cloneBranch, err)
	}

	return checkMergeTargetClaims(repo, plumbing.NewRemoteReferenceName(defaultRemote, cloneBranch), cloneBranch, cfg, written)
}

func entryPaths(files []FileEntry) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	return paths
}
