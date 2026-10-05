// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// releaseFixture is a remote holding two owners' trees plus an unrecorded document:
//
//   - A (KollectInventory team-a/apps) records a resource file and its split index;
//   - B (KollectClusterInventory platform) records its own resource file and, adversarially, the set
//     manifest path A's cleanup also addresses;
//   - A's legacy JSON document is unrecorded.
type releaseFixture struct {
	remote     string
	ownerA     string
	ownerB     string
	candidates []string // A's cleanup candidates, as sink.cleanupCandidatePaths renders them
}

const (
	relATree     = "default/team-a/deployment/api.yaml"
	relAIndex    = "inventory/team-a/apps.yaml"
	relADoc      = "inventory/team-a/apps.json"
	relBTree     = "default/team-b/deployment/api.yaml"
	relBManifest = "inventory/team-a/apps.manifest.json"
)

func newReleaseFixture(t *testing.T) releaseFixture {
	t.Helper()
	skipWithoutGit(t)

	ownerA, err := InventoryPruneOwner("KollectInventory", "default", "team-a", "apps")
	if err != nil {
		t.Fatal(err)
	}
	ownerB, err := InventoryPruneOwner("KollectClusterInventory", "default", "", "platform")
	if err != nil {
		t.Fatal(err)
	}
	f := releaseFixture{
		remote: createBareRemoteWithMainCommit(t), ownerA: ownerA, ownerB: ownerB,
		candidates: []string{relAIndex, relBManifest, relADoc},
	}
	f.export(t, "", false, FileEntry{Path: relADoc, Data: []byte(`{"legacy":"a"}`)})
	f.export(t, ownerA, true, FileEntry{Path: relATree, Data: []byte("a-tree\n")}, FileEntry{Path: relAIndex, Data: []byte("a-index\n")})
	f.export(t, ownerB, true, FileEntry{Path: relBTree, Data: []byte("b-tree\n")}, FileEntry{Path: relBManifest, Data: []byte("b-manifest\n")})

	return f
}

func (f releaseFixture) cfg() Config {
	return Config{Endpoint: "file://" + f.remote}.withDefaults()
}

func (f releaseFixture) export(t *testing.T, owner string, prune bool, files ...FileEntry) {
	t.Helper()
	cfg := f.cfg()
	cfg.PruneOwner, cfg.Prune = owner, prune
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContextFromObjectPath(files[0].Path, "default")); err != nil {
		t.Fatalf("seed export (owner %q): %v", owner, err)
	}
}

// release runs one ownership-aware deletion on the named engine.
func (f releaseFixture) release(t *testing.T, engine string, paths []string, opts ReleaseOptions) ([]string, error) {
	t.Helper()
	commitCtx := CommitContextFromObjectPath(relADoc, "default")
	if engine == "cli" {
		return DeleteExportWithBranch(t.Context(), ReleaseConfig(f.cfg(), opts), Auth{}, paths, nil, commitCtx)
	}
	cfg := deletionConfig(ReleaseConfig(f.cfg(), opts))
	req, validated, err := validateDeletePaths(cfg, paths, nil)
	if err != nil {
		return nil, err
	}

	return deleteRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx)
}

// tree maps every file on main to its content.
func (f releaseFixture) tree(t *testing.T) map[string]string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", f.remote, "ls-tree", "-r", "--name-only", "main").CombinedOutput() //nolint:gosec // G204: local test fixture
	if err != nil {
		t.Fatalf("ls-tree: %s: %v", out, err)
	}
	files := map[string]string{}
	for _, name := range strings.Fields(string(out)) {
		data, ok := remoteFile(t, f.remote, name)
		if !ok {
			t.Fatalf("show %s failed", name)
		}
		files[name] = string(data)
	}

	return files
}

func (f releaseFixture) commits(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", f.remote, "rev-list", "--count", "main").CombinedOutput() //nolint:gosec // G204: local test fixture
	if err != nil {
		t.Fatalf("rev-list: %s: %v", out, err)
	}

	return strings.TrimSpace(string(out))
}

func (f releaseFixture) lastSubject(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", f.remote, "log", "-1", "--format=%s", "main").CombinedOutput() //nolint:gosec // G204: local test fixture
	if err != nil {
		t.Fatalf("log: %s: %v", out, err)
	}

	return strings.TrimSpace(string(out))
}

// diffTrees lists the paths whose presence or bytes differ.
func diffTrees(before, after map[string]string) []string {
	var changed []string
	for p, v := range before {
		if w, ok := after[p]; !ok || w != v {
			changed = append(changed, p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			changed = append(changed, p)
		}
	}
	sort.Strings(changed)

	return changed
}

var releaseEngines = []string{"cli", "go-git"}

// ROD-1, ROD-2, ROD-3: Retain releases exactly the deleting owner's record in one commit and keeps
// every exported file and the other owner's record.
func TestReleaseExport_retainRemovesOnlyOwnRecord(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			before, commits := f.tree(t), f.commits(t)

			removed, err := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA, KeepFiles: true})
			if err != nil {
				t.Fatalf("release: %v", err)
			}
			after := f.tree(t)
			recordA := pruneRecordPath(f.ownerA)
			if got := diffTrees(before, after); len(got) != 1 || got[0] != recordA {
				t.Fatalf("Retain release changed %v, want exactly A's record %s (FORBIDDEN: Retain deletes an exported file)", got, recordA)
			}
			if _, ok := after[recordA]; ok {
				t.Fatalf("FORBIDDEN: A's record %s survived the release", recordA)
			}
			if len(removed) != 1 || removed[0] != recordA {
				t.Fatalf("removed = %v, want [%s]", removed, recordA)
			}
			if got, want := f.commits(t), commits; got == want {
				t.Fatalf("no release commit (rev-list --count still %s)", got)
			}
			if subject := f.lastSubject(t); !strings.Contains(subject, "release inventory ownership record") {
				t.Fatalf("release commit subject = %q", subject)
			}
		})
	}
}

// ROD-1, ROD-3, ROD-4: Delete removes A's candidates, A's recorded tree and A's record in one commit,
// and keeps the candidate B's record lists, B's tree and B's record byte for byte.
func TestReleaseExport_deleteRetractsOwnTreeKeepsForeign(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			before := f.tree(t)
			commits := f.commits(t)

			if _, err := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA}); err != nil {
				t.Fatalf("release: %v", err)
			}
			after := f.tree(t)
			want := []string{relADoc, relAIndex, relATree, pruneRecordPath(f.ownerA)}
			sort.Strings(want)
			if got := diffTrees(before, after); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("Delete release changed %v, want exactly %v", got, want)
			}
			for _, kept := range []string{relBManifest, relBTree, pruneRecordPath(f.ownerB)} {
				if after[kept] != before[kept] {
					t.Fatalf("FORBIDDEN: Delete release removed or rewrote %s, which B owns", kept)
				}
			}
			if got := f.commits(t); got == commits {
				t.Fatal("no retraction commit")
			}
			if n := countAfter(t, f, commits); n != 1 {
				t.Fatalf("retraction made %d commits, want 1", n)
			}
		})
	}
}

func countAfter(t *testing.T, f releaseFixture, before string) int {
	t.Helper()
	b, err := strconv.Atoi(before)
	if err != nil {
		t.Fatal(err)
	}
	a, err := strconv.Atoi(f.commits(t))
	if err != nil {
		t.Fatal(err)
	}

	return a - b
}

// plantRecord commits data at name on main by hand, as a person editing the repository would.
func plantRecord(t *testing.T, remote, name string, data []byte) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "plant")
	runGit(t, "clone", "--branch", "main", remote, work)
	runGitC(t, work, "config", "user.name", "Kollect Tests")
	runGitC(t, work, "config", "user.email", "kollect-tests@example.com")
	mustWriteFile(t, filepath.Join(work, filepath.FromSlash(name)), data)
	runGitC(t, work, "add", ".")
	runGitC(t, work, "commit", "-m", "hand edit")
	runGitC(t, work, "push", "origin", "main")
}

// ROD-5: an owner without a record (or one already released) is done: no error, no commit.
func TestReleaseExport_absentRecordIsNoop(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			stranger, err := InventoryPruneOwner("KollectInventory", "default", "team-z", "never")
			if err != nil {
				t.Fatal(err)
			}
			before, commits := f.tree(t), f.commits(t)
			for _, opts := range []ReleaseOptions{{Owner: stranger, KeepFiles: true}, {Owner: stranger}} {
				removed, relErr := f.release(t, engine, []string{"inventory/team-z/never.yaml"}, opts)
				if relErr != nil || len(removed) != 0 {
					t.Fatalf("release of an absent record (%+v) = %v, %v; want nothing removed, no error", opts, removed, relErr)
				}
			}
			if got := diffTrees(before, f.tree(t)); len(got) != 0 || f.commits(t) != commits {
				t.Fatalf("absent-record release changed %v / committed", got)
			}

			// Retry after a successful release: no error, no second commit.
			if _, relErr := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA, KeepFiles: true}); relErr != nil {
				t.Fatalf("first release: %v", relErr)
			}
			released := f.commits(t)
			if _, relErr := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA, KeepFiles: true}); relErr != nil {
				t.Fatalf("FORBIDDEN: retried release failed on an absent record: %v", relErr)
			}
			if f.commits(t) != released {
				t.Fatal("retried release committed again")
			}
		})
	}
}

// ROD-4: a deletion without an owner never deletes a path some record lists.
func TestDeleteExport_ownerlessKeepsRecordedPaths(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			before := f.tree(t)
			if _, err := f.release(t, engine, f.candidates, ReleaseOptions{}); err != nil {
				t.Fatalf("delete: %v", err)
			}
			got := diffTrees(before, f.tree(t))
			if len(got) != 1 || got[0] != relADoc {
				t.Fatalf("ownerless delete changed %v, want only the unrecorded %s (FORBIDDEN: deletes a recorded path)", got, relADoc)
			}
		})
	}
}

// ROD-3: a file at the deleting owner's record path that names another owner is not that owner's
// record; the release refuses it terminally and changes nothing.
func TestReleaseExport_foreignRecordAtOwnPathRefused(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			stranger, err := InventoryPruneOwner("KollectInventory", "default", "team-z", "never")
			if err != nil {
				t.Fatal(err)
			}
			// Plant B's record bytes at the stranger's record path.
			bRecord, ok := remoteFile(t, f.remote, pruneRecordPath(f.ownerB))
			if !ok {
				t.Fatal("B's record missing")
			}
			plantRecord(t, f.remote, pruneRecordPath(stranger), bRecord)
			before, commits := f.tree(t), f.commits(t)

			_, relErr := f.release(t, engine, nil, ReleaseOptions{Owner: stranger, KeepFiles: true})
			if relErr == nil || !kollecterrors.IsTerminal(relErr) {
				t.Fatalf("release of a record naming another owner: err = %v, want terminal", relErr)
			}
			if got := diffTrees(before, f.tree(t)); len(got) != 0 || f.commits(t) != commits {
				t.Fatalf("FORBIDDEN: refused release changed %v", got)
			}
		})
	}
}

// ROD-4 (legacy directory prune): an ownerless tree export with prune keeps a path another owner
// records in the same directory.
func TestExportFiles_legacyDirectoryPruneKeepsRecordedPaths(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			cfg := f.cfg()
			cfg.Prune = true
			files := []FileEntry{{Path: "default/team-b/deployment/web.yaml", Data: []byte("legacy\n")}}
			if engine == "cli" {
				if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{}); err != nil {
					t.Fatalf("legacy export: %v", err)
				}
			} else {
				req, validated, err := validateExportFiles(cfg, files, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := exportRemote(t.Context(), cfg, Auth{}, req, validated, CommitContext{}); err != nil {
					t.Fatalf("legacy export: %v", err)
				}
			}
			if data, ok := remoteFile(t, f.remote, relBTree); !ok || string(data) != "b-tree\n" {
				t.Fatalf("FORBIDDEN: legacy directory prune deleted %s, which B's record lists", relBTree)
			}
		})
	}
}

// ROD-2 / design D2: a Retain release reads only the deleting owner's record, so a damaged record of
// another inventory does not hold it. A Delete retraction must know every claim, so the same damage
// makes it terminal and it removes nothing.
func TestReleaseExport_damagedForeignRecord(t *testing.T) {
	for _, engine := range releaseEngines {
		t.Run(engine, func(t *testing.T) {
			f := newReleaseFixture(t)
			damaged := ".kollect-prune/" + strings.Repeat("ab", 32) + ".json"
			plantRecord(t, f.remote, damaged, []byte("{not json"))
			before, commits := f.tree(t), f.commits(t)

			_, delErr := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA})
			if delErr == nil || !kollecterrors.IsTerminal(delErr) {
				t.Fatalf("Delete over a damaged record: err = %v, want terminal", delErr)
			}
			if got := diffTrees(before, f.tree(t)); len(got) != 0 || f.commits(t) != commits {
				t.Fatalf("refused Delete changed %v", got)
			}

			if _, err := f.release(t, engine, f.candidates, ReleaseOptions{Owner: f.ownerA, KeepFiles: true}); err != nil {
				t.Fatalf("Retain release blocked by another inventory's damaged record: %v", err)
			}
			got := diffTrees(before, f.tree(t))
			if len(got) != 1 || got[0] != pruneRecordPath(f.ownerA) {
				t.Fatalf("Retain release changed %v, want only A's record (the damaged record must stay)", got)
			}
		})
	}
}
