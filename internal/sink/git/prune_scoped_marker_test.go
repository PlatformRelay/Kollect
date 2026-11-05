// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// assertInventoryScopedTerminal fails unless err is Terminal and carries the inventory-scoped marker
// (SCB-1: the per-sink circuit breaker must not count it) and its text contains want.
func assertInventoryScopedTerminal(t *testing.T, err error, want string) {
	t.Helper()
	if !kollecterrors.IsInventoryScoped(err) || !kollecterrors.IsTerminal(err) {
		t.Fatalf("err = %v (class %s, inventory-scoped %v), want terminal and inventory-scoped",
			err, kollecterrors.ClassOf(err), kollecterrors.IsInventoryScoped(err))
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want text containing %q", err, want)
	}
}

// TestOwnershipConflict_everyRoutedSiteIsInventoryScoped (B1.1): each raise site routed through
// ownershipConflict is driven through the function that raises it and must come back Terminal and
// inventory-scoped. Reverting any one site to invalidPrune fails its row. The last row is the
// control: a rejection about shared prune metadata stays unmarked, so the breaker still counts it.
func TestOwnershipConflict_everyRoutedSiteIsInventoryScoped(t *testing.T) {
	t.Parallel()
	const shared = "prod/team-a/Deployment/api.yaml"
	tooManyPaths := make([]string, maxPrunePaths+1)
	for i := range tooManyPaths {
		tooManyPaths[i] = fmt.Sprintf("p/%d.yaml", i)
	}
	for name, tc := range map[string]struct {
		site    string // the raise site at the time of writing, for the reader
		setup   func(t *testing.T, fs billy.Filesystem)
		cfg     Config
		written []string
		want    string // substring of the rejection
		scoped  bool
	}{
		"owner too long": {
			site: "prune_owned.go:174", cfg: Config{Prune: true, PruneOwner: strings.Repeat("o", 4097)},
			written: []string{"own.yaml"}, want: "prune owner exceeds 4096 bytes", scoped: true,
		},
		"keep set too large": {
			site: "prune_owned.go:268", cfg: Config{Prune: true, PruneOwner: "b", PruneKeepPaths: tooManyPaths},
			written: []string{"p/0.yaml"}, want: "prune inventory exceeds", scoped: true,
		},
		"written path missing from keep set": {
			site: "prune_owned.go:275", cfg: Config{Prune: true, PruneOwner: "b", PruneKeepPaths: []string{"keep.yaml"}},
			written: []string{"other.yaml"}, want: "missing from prune keep set", scoped: true,
		},
		"foreign claim on a written path (no prune)": {
			site:  "prune_owned.go:317 via checkForeignClaim",
			setup: func(t *testing.T, fs billy.Filesystem) { commitOwnedPaths(t, fs, "a", shared) },
			cfg:   Config{PruneOwner: "b"}, written: []string{shared},
			want: "belongs to another inventory", scoped: true,
		},
		"foreign claim on a later part's claim path": {
			site:  "prune_owned.go:317 via checkForeignClaim (PruneClaimPaths)",
			setup: func(t *testing.T, fs billy.Filesystem) { commitOwnedPaths(t, fs, "a", shared) },
			cfg:   Config{PruneOwner: "b", PruneClaimPaths: []string{shared}}, written: []string{"own.yaml"},
			want: "belongs to another inventory", scoped: true,
		},
		"record-advancing claim, other owner sorts first": {
			site:  "prune_owned.go:340 (checkSingleClaims)",
			setup: func(t *testing.T, fs billy.Filesystem) { commitOwnedPaths(t, fs, "a", shared) },
			cfg:   Config{Prune: true, PruneOwner: "b"}, written: []string{shared},
			want: "belongs to another inventory", scoped: true,
		},
		"record-advancing claim, exporter sorts first": {
			site:  "prune_owned.go:342 (checkSingleClaims)",
			setup: func(t *testing.T, fs billy.Filesystem) { commitOwnedPaths(t, fs, "b", shared) },
			cfg:   Config{Prune: true, PruneOwner: "a"}, written: []string{shared},
			want: "belongs to another inventory", scoped: true,
		},
		"control: shared metadata is not a directory": {
			site:  "prune_owned.go:135 (invalidPrune, sink-wide)",
			setup: func(t *testing.T, fs billy.Filesystem) { mustWriteBillyFile(t, fs, pruneMetadataDir, "x") },
			cfg:   Config{Prune: true, PruneOwner: "b"}, written: []string{"own.yaml"},
			want: "must be a real directory",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fs := memfs.New()
			if tc.setup != nil {
				tc.setup(t, fs)
			}
			_, err := prepareOwnedPrune(fs, tc.cfg, tc.written)
			if tc.scoped {
				assertInventoryScopedTerminal(t, err, tc.want)
				return
			}
			if !kollecterrors.IsTerminal(err) || kollecterrors.IsInventoryScoped(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: err = %v (inventory-scoped %v), want an unmarked terminal %q",
					tc.site, err, kollecterrors.IsInventoryScoped(err), tc.want)
			}
		})
	}

	t.Run("merge target claim", func(t *testing.T) { // prune_target.go:66 (checkMergeTargetClaims)
		t.Parallel()
		fs := memfs.New()
		repo, err := git.Init(memory.NewStorage(), fs)
		if err != nil {
			t.Fatal(err)
		}
		commitOwnedPaths(t, fs, "a", shared)
		wt, err := repo.Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if addErr := wt.AddGlob("."); addErr != nil {
			t.Fatal(addErr)
		}
		sig := &object.Signature{Name: "Kollect Tests", Email: "kollect-tests@example.com", When: time.Now()}
		if _, commitErr := wt.Commit("target", &git.CommitOptions{Author: sig}); commitErr != nil {
			t.Fatal(commitErr)
		}
		head, err := repo.Head()
		if err != nil {
			t.Fatal(err)
		}
		err = checkMergeTargetClaims(repo, head.Name(), "main", Config{Prune: true, PruneOwner: "b"}, []string{shared})
		assertInventoryScopedTerminal(t, err, "belongs to another inventory on merge target branch")
	})
}

// TestOwnershipConflict_goGitEngineKeepsMarker (B1.1): the go-git engine (exportRemoteInWorkdir, the
// engine for every non-file:// remote) returns an ownership rejection that is still Terminal and
// inventory-scoped after ClassifyExportError, the classifier ExportFilesWithBranch and
// RunExportEnvelope apply. SCB-1e covers the CLI engine (a file:// remote routes there); this drives
// the go-git engine directly against a local bare repository, offline, as the warm-mirror tests in
// prune_merge_target_test.go do.
func TestOwnershipConflict_goGitEngineKeepsMarker(t *testing.T) {
	const (
		shared  = "prod/team-a/Deployment/api.yaml"
		ownPath = "prod/team-b/Deployment/worker.yaml"
	)
	for name, tc := range map[string]struct {
		branch *BranchSpec
		want   string
	}{
		"same branch (prepareOwnedPrune)":       {want: "belongs to another inventory"},
		"merge target (checkMergeTargetClaims)": {branch: &BranchSpec{PushBranch: "kollect/team-b/apps", CloneBranch: "main"}, want: `on merge target branch "main"`},
	} {
		t.Run(name, func(t *testing.T) {
			skipWithoutGit(t)
			remote := createBareRemoteWithMainCommit(t)
			cloneURL := "file://" + remote
			a, _ := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
			b, _ := InventoryPruneOwner("KollectInventory", "default", "team-b", "apps")
			aCfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: a}.withDefaults()
			if err := ExportFilesWithBranch(t.Context(), aCfg, Auth{}, []FileEntry{{Path: shared, Data: []byte("a")}}, nil, CommitContext{}); err != nil {
				t.Fatalf("A's export: %v", err)
			}
			cfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: b}.withDefaults()
			files := []FileEntry{{Path: ownPath, Data: []byte("b")}, {Path: shared, Data: []byte("b")}}
			req, validated, err := validateExportFiles(cfg, files, tc.branch)
			if err != nil {
				t.Fatal(err)
			}
			workdir := filepath.Join(t.TempDir(), "mirror")
			err = ClassifyExportError(exportRemoteInWorkdir(t.Context(), cfg, nil, req, validated, CommitContext{}, workdir))
			assertInventoryScopedTerminal(t, err, tc.want)
		})
	}
}
