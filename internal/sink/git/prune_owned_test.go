// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

func ownedFilesystems(t *testing.T, run func(*testing.T, billy.Filesystem)) {
	t.Helper()
	t.Run("memory", func(t *testing.T) { run(t, memfs.New()) })
	t.Run("disk", func(t *testing.T) { run(t, osfs.New(t.TempDir())) })
}

func commitOwnedPaths(t *testing.T, fs billy.Filesystem, owner string, paths ...string) {
	t.Helper()
	cfg := Config{Prune: true, PruneOwner: owner}
	plan, err := prepareOwnedPrune(fs, cfg, paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		mustWriteBillyFile(t, fs, p, "data")
	}
	if err := plan.apply(fs); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedPrune_LastKindAndOwnerIsolation(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		old := "prod/team-a/Deployment/api.yaml"
		keep := "prod/team-a/Service/web.yaml"
		other := "prod/team-a/Secret/other.yaml"
		unknown := "prod/team-a/Deployment/historical.yaml"
		mustWriteBillyFile(t, fs, unknown, "unknown")
		commitOwnedPaths(t, fs, "inventory-b", other)
		commitOwnedPaths(t, fs, "inventory-a", old, keep)
		before, err := util.ReadFile(fs, pruneRecordPath("inventory-b"))
		if err != nil {
			t.Fatal(err)
		}
		commitOwnedPaths(t, fs, "inventory-a", keep)
		assertBillyExists(t, fs, old, false)
		for _, p := range []string{keep, other, unknown} {
			assertBillyExists(t, fs, p, true)
		}
		after, err := util.ReadFile(fs, pruneRecordPath("inventory-b"))
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("other owner's record changed")
		}
		commitOwnedPaths(t, fs, "inventory-a")
		assertBillyExists(t, fs, keep, false)
		assertBillyExists(t, fs, other, true)
	})
}

func TestOwnedPrune_MultipartDoesNotAdvanceRecordBeforeFinalUnion(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		commitOwnedPaths(t, fs, "inventory-a", "deep/a/old.yaml", "deep/b/keep.yaml")
		before, err := util.ReadFile(fs, pruneRecordPath("inventory-a"))
		if err != nil {
			t.Fatal(err)
		}
		plan, err := prepareOwnedPrune(fs, Config{PruneOwner: "inventory-a"}, []string{"deep/b/keep.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		if applyErr := plan.apply(fs); applyErr != nil {
			t.Fatal(applyErr)
		}
		after, err := util.ReadFile(fs, pruneRecordPath("inventory-a"))
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("suppressed part advanced record")
		}
		assertBillyExists(t, fs, "deep/a/old.yaml", true)
		cfg := Config{Prune: true, PruneOwner: "inventory-a", PruneKeepPaths: []string{"deep/b/keep.yaml", "deep/c/new.yaml"}}
		plan, err = prepareOwnedPrune(fs, cfg, []string{"deep/c/new.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		mustWriteBillyFile(t, fs, "deep/c/new.yaml", "new")
		if applyErr := plan.apply(fs); applyErr != nil {
			t.Fatal(applyErr)
		}
		assertBillyExists(t, fs, "deep/a/old.yaml", false)
		assertBillyExists(t, fs, "deep/b/keep.yaml", true)
		assertBillyExists(t, fs, "deep/c/new.yaml", true)
	})
}

func TestOwnedPrune_RejectsInvalidRecordsBeforeDeletion(t *testing.T) {
	for name, data := range map[string]string{
		"malformed": "{",
		"traversal": `{"version":1,"owner":"a","paths":["safe/old.yaml","../outside"]}`,
		"git":       `{"version":1,"owner":"a","paths":["safe/old.yaml",".git/config"]}`,
		"reserved":  `{"version":1,"owner":"a","paths":[".kollect-prune/other.json"]}`,
		"owner":     `{"version":1,"owner":"b","paths":["safe/old.yaml"]}`,
		"version":   `{"version":2,"owner":"a","paths":["safe/old.yaml"]}`,
		"duplicate": `{"version":1,"owner":"a","paths":["safe/old.yaml","safe/old.yaml"]}`,
		"trailing":  `{"version":1,"owner":"a","paths":[]} {}`,
		"unknown":   `{"version":1,"owner":"a","paths":[],"future":true}`,
		"large":     strings.Repeat("x", maxPruneMetadataBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			fs := memfs.New()
			mustWriteBillyFile(t, fs, "safe/old.yaml", "old")
			mustWriteBillyFile(t, fs, pruneRecordPath("a"), data)
			if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "a"}, []string{"safe/new.yaml"}); !kollecterrors.IsTerminal(err) {
				t.Fatalf("unsafe record error = %v, want terminal", err)
			}
			assertBillyExists(t, fs, "safe/old.yaml", true)
		})
	}
}

func TestOwnedPrune_RejectsSymlinksAndOwnerCollisions(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		commitOwnedPaths(t, fs, "a", "deep/team-a/old.yaml")
		if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "b"}, []string{"deep/team-a/old.yaml"}); err == nil {
			t.Fatal("another owner's file claimed")
		}
		if err := fs.Symlink("deep/team-a", "link"); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "a"}, []string{"link/new.yaml"}); err == nil {
			t.Fatal("symlink parent accepted")
		}
		record := pruneRecordPath("a")
		data, err := json.Marshal(pruneRecord{Version: 1, Owner: "a", Paths: []string{"link/old.yaml"}})
		if err != nil {
			t.Fatal(err)
		}
		mustWriteBillyFile(t, fs, record, string(data))
		if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "a"}, []string{"deep/team-a/new.yaml"}); err == nil {
			t.Fatal("symlink in old ownership accepted")
		}
		assertBillyExists(t, fs, "deep/team-a/old.yaml", true)
	})
	t.Run("metadata symlink", func(t *testing.T) {
		fs := osfs.New(t.TempDir())
		outside := t.TempDir()
		if err := fs.Symlink(outside, pruneMetadataDir); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "a"}, []string{"safe/new.yaml"}); err == nil {
			t.Fatal("metadata symlink accepted")
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("outside changed: %v %v", entries, err)
		}
	})
}

func TestOwnedPrune_RecordDeterministicAndMissingHistoryPreserved(t *testing.T) {
	fs := memfs.New()
	mustWriteBillyFile(t, fs, "custom/deep/team-b/unmanaged.yaml", "historical")
	commitOwnedPaths(t, fs, "a", "custom/deep/team-a/b.yaml", "custom/deep/team-a/a.yaml")
	before, err := util.ReadFile(fs, pruneRecordPath("a"))
	if err != nil {
		t.Fatal(err)
	}
	commitOwnedPaths(t, fs, "a", "custom/deep/team-a/a.yaml", "custom/deep/team-a/b.yaml")
	after, err := util.ReadFile(fs, pruneRecordPath("a"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("ownership record depends on export order")
	}
	assertBillyExists(t, fs, "custom/deep/team-b/unmanaged.yaml", true)
}

func TestOwnedPrune_RejectsUnsafeCurrentPathsAndMissingUnion(t *testing.T) {
	for _, cfg := range []Config{
		{Prune: true, PruneOwner: "a", PruneKeepPaths: []string{"../outside"}},
		{Prune: true, PruneOwner: "a", PruneKeepPaths: []string{"different.yaml"}},
	} {
		if _, err := prepareOwnedPrune(memfs.New(), cfg, []string{"safe.yaml"}); err == nil {
			t.Fatal("unsafe union accepted")
		}
	}
}

func TestOwnedPrune_BoundsNextSnapshotUsingStoredBytes(t *testing.T) {
	fs := memfs.New()
	record := `{"version":1,"owner":"b","paths":[]}`
	record += strings.Repeat(" ", maxPruneMetadataBytes-len(record)-1)
	mustWriteBillyFile(t, fs, pruneRecordPath("b"), record)
	if _, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: "a"}, []string{"new.yaml"}); !kollecterrors.IsTerminal(err) {
		t.Fatalf("oversized next snapshot: %v", err)
	}
}

func TestOwnedPrune_RejectsMalformedMetadataBeforeOverwrite(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		mustWriteBillyFile(t, fs, "safe/old.yaml", "original")
		mustWriteBillyFile(t, fs, pruneRecordPath("a"), "{")
		_, err := writeBillyExportFiles(fs, Config{Prune: true, PruneOwner: "a"}, []FileEntry{{Path: "safe/old.yaml", Data: []byte("changed")}})
		if !kollecterrors.IsTerminal(err) {
			t.Fatalf("malformed metadata error: %v", err)
		}
		data, err := util.ReadFile(fs, "safe/old.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Fatal("file modified before metadata validation")
		}
	})
}
