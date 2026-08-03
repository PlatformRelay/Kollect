// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// exportRemote is the go-git-native push path (as opposed to exportViaCLI). The high-level
// ExportFilesWithBranch entry point always routes file:// remotes through the CLI path, so
// exportRemote is exercised here directly with a local bare repo to cover the in-memory
// worktree write/commit/push flow without requiring a real network-reachable git host.
func TestExportRemote_PushesFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	files := []FileEntry{{Path: "inventory/latest.json", Data: []byte(`{"hello":"world"}`)}}
	req, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
		t.Fatalf("exportRemote() error = %v", exportErr)
	}

	verify := t.TempDir()
	if out, cloneErr := exec.Command("git", "clone", "--branch", "main", "--single-branch", remote, verify).CombinedOutput(); cloneErr != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone verify: %s: %v", out, cloneErr)
	}

	data, err := os.ReadFile(filepath.Join(verify, "inventory", "latest.json")) //nolint:gosec // G304: test fixture
	if err != nil {
		t.Fatalf("read inventory/latest.json: %v", err)
	}
	if string(data) != `{"hello":"world"}` {
		t.Fatalf("payload = %q", data)
	}
}

// TestExportRemote_MultipartUnionPrunePreservesPartsDropsStale exercises the go-git-native prune
// path (pruneBillyOrphans / pruneKeepSet union branch / stageChanges go-git delete branch). The
// high-level entry point routes file:// through the CLI engine, so the go-git union-prune is driven
// here directly against a local bare repo. It mirrors the multipart controller flow: a non-final
// part writes with prune suppressed (Prune=false), then the final part prunes EXACTLY ONCE against
// the union of every part's paths. A prior part's file must survive (no data loss) while a resource
// dropped since the previous generation is pruned (union correctness).
func TestExportRemote_MultipartUnionPrunePreservesPartsDropsStale(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	const dir = "default/team-a/deployment"

	runPart := func(prune bool, keep []string, files ...FileEntry) {
		t.Helper()
		cfg := Config{Endpoint: "file://" + remote, Prune: prune, PruneKeepPaths: keep}.withDefaults()
		req, validated, err := validateExportFiles(cfg, files, nil)
		if err != nil {
			t.Fatalf("validateExportFiles: %v", err)
		}
		commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)
		if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
			t.Fatalf("exportRemote() error = %v", exportErr)
		}
	}

	api := FileEntry{Path: dir + "/api.yaml", Data: []byte("kind: Deployment\nmetadata:\n  name: api\n")}
	web := FileEntry{Path: dir + "/web.yaml", Data: []byte("kind: Deployment\nmetadata:\n  name: web\n")}
	db := FileEntry{Path: dir + "/db.yaml", Data: []byte("kind: Deployment\nmetadata:\n  name: db\n")}

	// Gen1: both resources present (single write, legacy keep-set = written paths).
	runPart(true, nil, api, web)

	// Gen2 multipart. Non-final part 1 writes api with prune SUPPRESSED (Prune=false) so it cannot
	// delete siblings; final part 2 writes db and prunes ONCE against the union {api, db}.
	runPart(false, nil, api)
	runPart(true, []string{api.Path, db.Path}, db)

	verify := t.TempDir()
	if out, cloneErr := exec.Command("git", "clone", "--branch", "main", "--single-branch", remote, verify).CombinedOutput(); cloneErr != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git clone verify: %s: %v", out, cloneErr)
	}

	// Prior part's file survives (no data loss) and the new resource is written.
	for _, rel := range []string{dir + "/api.yaml", dir + "/db.yaml"} {
		if _, err := os.Stat(filepath.Join(verify, rel)); err != nil {
			t.Fatalf("expected %s to survive the union prune: %v", rel, err)
		}
	}
	// The resource dropped since the previous generation is pruned exactly once, on the final part.
	if _, err := os.Stat(filepath.Join(verify, dir, "web.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected %s/web.yaml to be pruned, stat err=%v", dir, err)
	}
}

func TestExportRemote_NoChangesIsNoop(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	files := []FileEntry{{Path: "README.md", Data: []byte("seed\n")}}
	req, validated, err := validateExportFiles(cfg, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	commitCtx := CommitContextFromObjectPath(req.objectPath, cfg.Cluster)

	if exportErr := exportRemote(t.Context(), cfg, Auth{}, req, validated, commitCtx); exportErr != nil {
		t.Fatalf("exportRemote() error = %v, want clean-status no-op", exportErr)
	}
}

func TestCloneOrInit_emptyRemoteFallsBackToInit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	dir := t.TempDir()
	bare := filepath.Join(dir, "empty.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", bare).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	workdir := filepath.Join(dir, "work")
	repo, emptyRemote, err := cloneOrInit(t.Context(), workdir, "file://"+bare, "main", nil, Config{CloneDepth: 1})
	if err != nil {
		t.Fatalf("cloneOrInit() error = %v", err)
	}
	if !emptyRemote {
		t.Fatal("cloneOrInit() emptyRemote = false, want true for an empty bare repo")
	}
	if repo == nil {
		t.Fatal("cloneOrInit() repo = nil")
	}

	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		t.Fatalf(".git missing after init fallback: %v", err)
	}
}

func TestCloneOrInit_nonEmptyRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	remote := createBareRemoteWithMainCommit(t)
	workdir := filepath.Join(t.TempDir(), "work")

	repo, emptyRemote, err := cloneOrInit(t.Context(), workdir, "file://"+remote, "main", nil, Config{CloneDepth: 1})
	if err != nil {
		t.Fatalf("cloneOrInit() error = %v", err)
	}
	if emptyRemote {
		t.Fatal("cloneOrInit() emptyRemote = true, want false for seeded repo")
	}
	if repo == nil {
		t.Fatal("cloneOrInit() repo = nil")
	}
	if _, err := repo.Head(); err != nil {
		t.Fatalf("repo.Head() error = %v, want resolvable HEAD", err)
	}
}
