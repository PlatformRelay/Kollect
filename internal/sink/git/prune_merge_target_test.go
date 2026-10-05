// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// remoteRecords returns the ownership records on one branch of a bare remote, keyed by record path.
func remoteRecords(t *testing.T, remote, branch string) map[string]pruneRecord {
	t.Helper()
	records := make(map[string]pruneRecord)
	out, err := exec.Command("git", "--git-dir", remote, "ls-tree", "--name-only", branch+":"+pruneMetadataDir).CombinedOutput() //nolint:gosec // G204: local test fixture
	if err != nil {
		return records // no record directory on this branch
	}
	for _, name := range strings.Fields(string(out)) {
		recordPath := pruneMetadataDir + "/" + name
		data, found := remoteBranchFile(t, remote, branch, recordPath)
		if !found {
			t.Fatalf("record %s listed but unreadable on %s", recordPath, branch)
		}
		var record pruneRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatalf("decode %s on %s: %v", recordPath, branch, err)
		}
		records[recordPath] = record
	}
	return records
}

func remoteBranchFile(t *testing.T, remote, branch, name string) ([]byte, bool) {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", remote, "show", branch+":"+name).CombinedOutput() //nolint:gosec // G204: local test fixture
	return out, err == nil
}

func remoteTip(t *testing.T, remote, branch string) string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", remote, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Output() //nolint:gosec // G204: local test fixture
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// assertMergeClaimsUnique fails when merging feature into target would leave two records claiming
// one path. A feature branch only adds or rewrites its own inventory's record, so the merged record
// set is target's records with feature's version of every record it carries.
func assertMergeClaimsUnique(t *testing.T, remote, target, feature string) {
	t.Helper()
	merged := remoteRecords(t, remote, target)
	for recordPath, record := range remoteRecords(t, remote, feature) {
		merged[recordPath] = record
	}
	claimedBy := make(map[string]string)
	for _, record := range merged {
		for _, p := range record.Paths {
			if prev, dup := claimedBy[p]; dup && prev != record.Owner {
				t.Fatalf("FORBIDDEN: merging %s into %s claims %q twice: %s and %s", feature, target, p, prev, record.Owner)
			}
			claimedBy[p] = record.Owner
		}
	}
}

// TestBranchExport_warmMirrorRefusesClaimMergedOnTarget (IEI-10): a merge-request export is checked
// against the merge target's records, not only against the feature branch it builds on. B's feature
// branch predates A's merged claim; B's next export from the warm mirror (which checks out B's own,
// stale branch) must not push a claim on A's path, and must name A and A's record.
func TestBranchExport_warmMirrorRefusesClaimMergedOnTarget(t *testing.T) {
	const (
		shared  = "prod/team-a/Deployment/api.yaml"
		ownPath = "prod/team-b/Deployment/worker.yaml"
		feature = "kollect/team-b/apps"
	)
	for _, engine := range []string{"cli", "go-git"} {
		t.Run(engine, func(t *testing.T) {
			skipWithoutGit(t)
			remote := createBareRemoteWithMainCommit(t)
			cloneURL := "file://" + remote
			workdir := filepath.Join(t.TempDir(), "mirror") // kept across exports: warm after the first
			a, _ := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
			b, _ := InventoryPruneOwner("KollectInventory", "default", "team-b", "apps")
			branch := &BranchSpec{PushBranch: feature, CloneBranch: "main"}

			exportB := func(files ...FileEntry) error {
				t.Helper()
				cfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: b}.withDefaults()
				req, validated, err := validateExportFiles(cfg, files, branch)
				if err != nil {
					t.Fatal(err)
				}
				if engine == "go-git" {
					return exportRemoteInWorkdir(t.Context(), cfg, nil, req, validated, CommitContext{}, workdir)
				}
				cli, err := newCLIEnv(cfg, Auth{}, cfg.AuthType)
				if err != nil {
					t.Fatal(err)
				}
				defer cli.cleanup()
				return exportViaCLIInWorkdir(t.Context(), cfg, Auth{}, cli, workdir, req.cloneURL, req.cloneBranch, req.pushBranch, validated, CommitContext{})
			}

			// B opens its merge-request branch with a path nobody claims.
			if err := exportB(FileEntry{Path: ownPath, Data: []byte("b")}); err != nil {
				t.Fatalf("B's first export: %v", err)
			}
			// A's claim on the shared path reaches main (A's merge request merged).
			aCfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: a}.withDefaults()
			if err := ExportFilesWithBranch(t.Context(), aCfg, Auth{}, []FileEntry{{Path: shared, Data: []byte("a")}}, nil, CommitContext{}); err != nil {
				t.Fatalf("A's export to main: %v", err)
			}
			before := remoteTip(t, remote, feature)

			err := exportB(FileEntry{Path: ownPath, Data: []byte("b")}, FileEntry{Path: shared, Data: []byte("b")})
			assertMergeClaimsUnique(t, remote, "main", feature)
			if !kollecterrors.IsTerminal(err) {
				t.Fatalf("B claims A's merged path: err = %v, want a terminal rejection", err)
			}
			for _, want := range []string{
				shared, "belongs to another inventory", `KollectInventory team-a/apps (cluster "default")`,
				pruneRecordPath(a), `"main"`,
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rejection %q does not name %q", err, want)
				}
			}
			if after := remoteTip(t, remote, feature); after != before {
				t.Fatalf("refused export moved %s from %s to %s", feature, before, after)
			}

			// B's own claims stay exportable from the same warm mirror: a snapshot without the shared
			// path is accepted and pushed.
			if err := exportB(FileEntry{Path: ownPath, Data: []byte("b2")}); err != nil {
				t.Fatalf("B's export of its own path after the refusal: %v", err)
			}
			if data, ok := remoteBranchFile(t, remote, feature, ownPath); !ok || string(data) != "b2" {
				t.Fatalf("B's own path on %s = %q, %v, want b2", feature, data, ok)
			}
			assertMergeClaimsUnique(t, remote, "main", feature)
		})
	}
}

// TestSameBranchExport_warmMirrorRefusesClaimPushedMeanwhile: without a merge-request branch the CLI
// engine checked out its stale local branch, so a claim another inventory pushed meanwhile was
// invisible; the push was rejected as non-fast-forward and pull --rebase merged both records in
// (identical bytes rebase cleanly). The check must read the fetched tip in this mode too.
func TestSameBranchExport_warmMirrorRefusesClaimPushedMeanwhile(t *testing.T) {
	const (
		shared  = "prod/team-a/Deployment/api.yaml"
		ownPath = "prod/team-b/Deployment/worker.yaml"
	)
	for _, engine := range []string{"cli", "go-git"} {
		t.Run(engine, func(t *testing.T) {
			skipWithoutGit(t)
			remote := createBareRemoteWithMainCommit(t)
			cloneURL := "file://" + remote
			workdir := filepath.Join(t.TempDir(), "mirror")
			a, _ := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
			b, _ := InventoryPruneOwner("KollectInventory", "default", "team-b", "apps")

			exportB := func(files ...FileEntry) error {
				t.Helper()
				cfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: b}.withDefaults()
				req, validated, err := validateExportFiles(cfg, files, nil)
				if err != nil {
					t.Fatal(err)
				}
				if engine == "go-git" {
					return exportRemoteInWorkdir(t.Context(), cfg, nil, req, validated, CommitContext{}, workdir)
				}
				cli, err := newCLIEnv(cfg, Auth{}, cfg.AuthType)
				if err != nil {
					t.Fatal(err)
				}
				defer cli.cleanup()
				return exportViaCLIInWorkdir(t.Context(), cfg, Auth{}, cli, workdir, req.cloneURL, req.cloneBranch, req.pushBranch, validated, CommitContext{})
			}

			if err := exportB(FileEntry{Path: ownPath, Data: []byte("b")}); err != nil {
				t.Fatalf("B's first export: %v", err)
			}
			aCfg := Config{Endpoint: cloneURL, Prune: true, PruneOwner: a}.withDefaults()
			if err := ExportFilesWithBranch(t.Context(), aCfg, Auth{}, []FileEntry{{Path: shared, Data: []byte("same")}}, nil, CommitContext{}); err != nil {
				t.Fatalf("A's export: %v", err)
			}

			err := exportB(FileEntry{Path: ownPath, Data: []byte("b")}, FileEntry{Path: shared, Data: []byte("same")})
			assertMergeClaimsUnique(t, remote, "main", "main")
			if !kollecterrors.IsTerminal(err) {
				t.Fatalf("B claims the path A pushed meanwhile: err = %v, want a terminal rejection", err)
			}
			if !strings.Contains(err.Error(), "belongs to another inventory") {
				t.Fatalf("rejection %q does not name the ownership conflict", err)
			}
		})
	}
}

// TestLoadPruneRecords_duplicateNamesBothOwnersAndRecords (IEI-12): a branch that already holds two
// records claiming one path fails every export with an error an operator can act on: both owners
// and both record files.
func TestLoadPruneRecords_duplicateNamesBothOwnersAndRecords(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		const shared = "prod/team-a/Deployment/api.yaml"
		a, _ := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
		l, _ := InventoryPruneOwner("KollectClusterInventory", "default", "", "platform")
		for _, owner := range []string{a, l} {
			data, err := json.Marshal(pruneRecord{Version: 1, Owner: owner, Paths: []string{shared}})
			if err != nil {
				t.Fatal(err)
			}
			if err := util.WriteFile(fs, pruneRecordPath(owner), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}

		_, _, _, err := loadPruneRecords(fs)
		if !kollecterrors.IsTerminal(err) {
			t.Fatalf("duplicate claim: err = %v, want terminal", err)
		}
		for _, want := range []string{
			shared,
			`KollectInventory team-a/apps (cluster "default")`, pruneRecordPath(a),
			`KollectClusterInventory platform (cluster "default")`, pruneRecordPath(l),
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("duplicate-ownership error %q does not name %q", err, want)
			}
		}
	})
}

// TestCheckMergeTargetClaims_readsTargetTreeFailClosed (IEI-10): the target's records are read from
// its commit tree with the strict loader: a foreign claim and malformed metadata refuse the export,
// the exporter's own claim and a target without records or without the branch do not.
func TestCheckMergeTargetClaims_readsTargetTreeFailClosed(t *testing.T) {
	skipWithoutGit(t)
	const shared = "prod/team-a/Deployment/api.yaml"
	a, _ := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
	b, _ := InventoryPruneOwner("KollectInventory", "default", "team-b", "apps")
	record := func(owner string) []byte {
		data, err := json.Marshal(pruneRecord{Version: 1, Owner: owner, Paths: []string{shared}})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for name, tc := range map[string]struct {
		setup    func(work string)
		exporter string
		refused  string // substring of the terminal error; empty means accepted
		manifest bool   // the shared path is only the set manifest a later part writes
	}{
		"foreign claim on the set manifest": {
			setup:    func(work string) { mustWriteFile(t, filepath.Join(work, pruneRecordPath(a)), record(a)) },
			exporter: b, refused: "belongs to another inventory on merge target branch", manifest: true,
		},
		"foreign claim": {
			setup:    func(work string) { mustWriteFile(t, filepath.Join(work, pruneRecordPath(a)), record(a)) },
			exporter: b, refused: "belongs to another inventory on merge target branch",
		},
		"own claim": {
			setup:    func(work string) { mustWriteFile(t, filepath.Join(work, pruneRecordPath(b)), record(b)) },
			exporter: b,
		},
		"no records": {setup: func(string) {}, exporter: b},
		"symlinked record": {
			setup: func(work string) {
				mustWriteFile(t, filepath.Join(work, "elsewhere.json"), record(a))
				if err := os.MkdirAll(filepath.Join(work, pruneMetadataDir), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../elsewhere.json", filepath.Join(work, pruneRecordPath(a))); err != nil {
					t.Fatal(err)
				}
			},
			exporter: b, refused: "symlink",
		},
		"record directory is a file": {
			setup:    func(work string) { mustWriteFile(t, filepath.Join(work, pruneMetadataDir), []byte("x")) },
			exporter: b, refused: "real directory",
		},
	} {
		t.Run(name, func(t *testing.T) {
			work := filepath.Join(t.TempDir(), "work")
			runGit(t, "init", "-b", "main", work)
			runGitC(t, work, "config", "user.name", "Kollect Tests")
			runGitC(t, work, "config", "user.email", "kollect-tests@example.com")
			mustWriteFile(t, filepath.Join(work, "README.md"), []byte("seed\n"))
			tc.setup(work)
			runGitC(t, work, "add", "-A")
			runGitC(t, work, "commit", "-m", "target")
			repo, err := git.PlainOpen(work)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{Prune: true, PruneOwner: tc.exporter}
			written := []string{shared}
			if tc.manifest {
				cfg.Prune, cfg.PruneClaimPaths, written = false, []string{shared}, []string{"prod/team-b/Deployment/worker.yaml"}
			}
			err = checkMergeTargetClaims(repo, plumbing.NewBranchReferenceName("main"), "main", cfg, written)
			if tc.refused == "" {
				if err != nil {
					t.Fatalf("err = %v, want accepted", err)
				}
			} else if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), tc.refused) {
				t.Fatalf("err = %v, want a terminal error containing %q", err, tc.refused)
			}
			if missingErr := checkMergeTargetClaims(repo, plumbing.NewBranchReferenceName("absent"), "absent", cfg, []string{shared}); missingErr != nil {
				t.Fatalf("target branch absent (empty remote): err = %v, want nil", missingErr)
			}
		})
	}
}
