// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
)

func remoteFile(t *testing.T, remote, name string) ([]byte, bool) {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", remote, "show", "main:"+name).CombinedOutput() //nolint:gosec // G204: local test fixture
	return out, err == nil
}

func TestExportOwnedPrune_BothEnginesCommitExactOwnership(t *testing.T) {
	for _, engine := range []string{"cli", "go-git"} {
		t.Run(engine, func(t *testing.T) {
			remote := createBareRemoteWithMainCommit(t)
			run := func(owner string, prune bool, keep []string, files ...FileEntry) {
				t.Helper()
				cfg := Config{Endpoint: "file://" + remote, Prune: prune, PruneOwner: owner, PruneKeepPaths: keep}.withDefaults()
				if engine == "cli" {
					if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{}); err != nil {
						t.Fatal(err)
					}
				} else {
					req, validated, err := validateExportFiles(cfg, files, nil)
					if err != nil {
						t.Fatal(err)
					}
					if err := exportRemote(t.Context(), cfg, Auth{}, req, validated, CommitContext{}); err != nil {
						t.Fatal(err)
					}
				}
			}
			old := FileEntry{Path: "prod/team-a/Deployment/api.yaml", Data: []byte("old")}
			keep := FileEntry{Path: "prod/team-a/Service/web.yaml", Data: []byte("keep")}
			next := FileEntry{Path: "custom/deep/team-a/new.yaml", Data: []byte("next")}
			other := FileEntry{Path: "prod/team-a/Deployment/other.yaml", Data: []byte("other")}
			unknown := FileEntry{Path: "custom/deep/team-b/historical.yaml", Data: []byte("unknown")}
			run("", false, nil, unknown)
			run("b", true, nil, other)
			run("a", true, nil, old, keep)
			initial, found := remoteFile(t, remote, pruneRecordPath("a"))
			if !found {
				t.Fatalf("record not committed: %s", initial)
			}
			run("a", false, nil, keep)
			pending, pendingFound := remoteFile(t, remote, pruneRecordPath("a"))
			if !pendingFound || string(initial) != string(pending) {
				t.Fatal("non-final part changed record")
			}
			run("a", true, []string{keep.Path, next.Path}, next)
			if data, ok := remoteFile(t, remote, old.Path); ok {
				t.Fatalf("old kind survived: %s", data)
			}
			for _, f := range []FileEntry{keep, next, other, unknown} {
				if data, ok := remoteFile(t, remote, f.Path); !ok || string(data) != string(f.Data) {
					t.Fatalf("file %s lost or changed: %s", f.Path, data)
				}
			}
			data, ok := remoteFile(t, remote, pruneRecordPath("a"))
			if !ok {
				t.Fatal("record absent")
			}
			var record pruneRecord
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if len(record.Paths) != 2 || record.Paths[0] != next.Path || record.Paths[1] != keep.Path {
				t.Fatalf("union record = %#v", record)
			}
			run("a", true, nil)
			for _, f := range []FileEntry{keep, next} {
				if _, ok := remoteFile(t, remote, f.Path); ok {
					t.Fatalf("empty inventory retained %s", f.Path)
				}
			}
			for _, f := range []FileEntry{other, unknown} {
				if _, ok := remoteFile(t, remote, f.Path); !ok {
					t.Fatalf("empty inventory deleted %s", f.Path)
				}
			}
		})
	}
}

func TestBackend_ExportFilesPropagatesPruneOwner(t *testing.T) {
	remote := createBareRemoteWithMainCommit(t)
	backend := &Backend{cfg: Config{Endpoint: "file://" + remote}.withDefaults()}
	files := []FileEntry{{Path: "deep/team-a/old.yaml", Data: []byte("old")}}
	opts := ExportFilesOptions{Prune: true, PruneOwner: "a"}
	if err := backend.ExportFiles(t.Context(), files, opts); err != nil {
		t.Fatal(err)
	}
	if _, ok := remoteFile(t, remote, pruneRecordPath("a")); !ok {
		t.Fatal("backend did not persist ownership")
	}
	if err := backend.ExportFiles(t.Context(), nil, opts); err != nil {
		t.Fatal(err)
	}
	if _, ok := remoteFile(t, remote, files[0].Path); ok {
		t.Fatal("empty tree did not prune prior ownership")
	}
}

func TestExportOwnedPrune_FingerprintIncludesOwnerAndKeepSet(t *testing.T) {
	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote, Prune: true, PruneOwner: "a"}.withDefaults()
	keep := FileEntry{Path: "deep/keep.yaml", Data: []byte("keep")}
	old := FileEntry{Path: "deep/old.yaml", Data: []byte("old")}
	commit := CommitContext{Checksum: "same-checksum"}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{keep, old}, nil, commit); err != nil {
		t.Fatal(err)
	}
	other := cfg
	other.PruneOwner = "b"
	if err := ExportFilesWithBranch(t.Context(), other, Auth{}, []FileEntry{keep}, nil, commit); err == nil {
		t.Fatal("fingerprint skipped another owner's collision")
	}
	if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{keep}, nil, commit); err != nil {
		t.Fatal(err)
	}
	if _, ok := remoteFile(t, remote, old.Path); ok {
		t.Fatal("fingerprint skipped changed keep-set")
	}
}

func TestExportOwnedPruneFingerprintReturnToPriorSet(t *testing.T) {
	for _, firstChanges := range []bool{false, true} {
		t.Run(fmt.Sprint(firstChanges), func(t *testing.T) {
			remote := createBareRemoteWithMainCommit(t)
			cfg := Config{Endpoint: "file://" + remote, Prune: true, PruneOwner: "a"}.withDefaults()
			keep := FileEntry{Path: "deep/keep.yaml", Data: []byte("keep")}
			old := FileEntry{Path: "deep/old.yaml", Data: []byte("old")}
			initial := []FileEntry{keep, old}
			if firstChanges {
				initial = []FileEntry{old, keep}
			}
			for _, snapshot := range []struct {
				files    []FileEntry
				checksum string
			}{{initial, "A"}, {[]FileEntry{keep}, "B"}, {initial, "A"}} {
				if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, snapshot.files, nil, CommitContext{Checksum: snapshot.checksum}); err != nil {
					t.Fatal(err)
				}
			}
			if _, ok := remoteFile(t, remote, old.Path); !ok {
				t.Fatal("A->B->A failed to restore removed file")
			}
		})
	}
}

// A non-final part must replace the latest cached operation even if the final
// part previously had the same checksum and keep-set.
func TestExportOwnedPruneFingerprintMultipartReplay(t *testing.T) {
	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote, PruneOwner: "a"}.withDefaults()
	keep := FileEntry{Path: "deep/keep.yaml", Data: []byte("keep")}
	part := FileEntry{Path: keep.Path, Data: []byte("part")}
	run := func(prune bool, checksum string, files ...FileEntry) {
		t.Helper()
		cfg.Prune = prune
		if err := ExportFilesWithBranch(t.Context(), cfg, Auth{}, files, nil, CommitContext{Checksum: checksum}); err != nil {
			t.Fatal(err)
		}
	}
	run(false, "part", part)
	run(true, "final", keep)
	run(false, "part", part)
	if got, _ := remoteFile(t, remote, keep.Path); string(got) != "part" {
		t.Fatalf("replayed non-final part skipped: %q", got)
	}
	run(true, "final", keep)
	if got, _ := remoteFile(t, remote, keep.Path); string(got) != "keep" {
		t.Fatalf("replayed final part skipped: %q", got)
	}
	// A write without a trusted checksum must also invalidate an earlier hit.
	run(true, "", FileEntry{Path: keep.Path, Data: []byte("changed")})
	run(true, "final", keep)
	if got, _ := remoteFile(t, remote, keep.Path); string(got) != "keep" {
		t.Fatalf("unchecked operation left stale cached state: %q", got)
	}
}
