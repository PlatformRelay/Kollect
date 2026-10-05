// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/git"
)

const (
	kindNamespacedInventory = "KollectInventory"
	kindClusterInventory    = "KollectClusterInventory"
)

type branchRecord struct {
	Version int      `json:"version"`
	Owner   string   `json:"owner"`
	Paths   []string `json:"paths"`
}

func gitRemoteOutput(t *testing.T, remote string, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"--git-dir", remote}, args...)...).Output() //nolint:gosec // G204: local test fixture
	return string(out), err
}

// remoteBranches lists the remote's branches under prefix.
func remoteBranches(t *testing.T, remote, prefix string) []string {
	t.Helper()
	out, err := gitRemoteOutput(t, remote, "for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads/"+prefix+"/")
	if err != nil {
		t.Fatalf("for-each-ref: %v", err)
	}
	return strings.Fields(out)
}

// branchRecords decodes every ownership record on one branch, keyed by record path.
func branchRecords(t *testing.T, remote, branch string) map[string]branchRecord {
	t.Helper()
	records := make(map[string]branchRecord)
	out, err := gitRemoteOutput(t, remote, "ls-tree", "--name-only", branch+":.kollect-prune")
	if err != nil {
		return records
	}
	for _, name := range strings.Fields(out) {
		data, showErr := gitRemoteOutput(t, remote, "show", branch+":.kollect-prune/"+name)
		if showErr != nil {
			t.Fatalf("show record %s on %s: %v", name, branch, showErr)
		}
		var record branchRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			t.Fatalf("decode record %s on %s: %v", name, branch, err)
		}
		records[".kollect-prune/"+name] = record
	}
	return records
}

// assertNoBranchDuplicatesOnMerge fails when any feature branch under prefix carries two records
// claiming one path, or would leave two such records on target once merged. A feature branch only
// adds or rewrites its own record, so the merged set is target's records overlaid with the branch's.
func assertNoBranchDuplicatesOnMerge(t *testing.T, remote, target, prefix string) {
	t.Helper()
	for _, branch := range remoteBranches(t, remote, prefix) {
		merged := branchRecords(t, remote, target)
		for recordPath, record := range branchRecords(t, remote, branch) {
			merged[recordPath] = record
		}
		claimedBy := make(map[string]string)
		for _, record := range merged {
			for _, p := range record.Paths {
				if prev, dup := claimedBy[p]; dup && prev != record.Owner {
					t.Fatalf("FORBIDDEN: branch %s merged into %s claims %q twice: %s and %s", branch, target, p, prev, record.Owner)
				}
				claimedBy[p] = record.Owner
			}
		}
	}
}

// branchMRExport runs one ExportFiles in merge_request mode and tolerates only the failure every
// file:// endpoint ends with: the merge-request API cannot be derived from it, which proves the push
// step finished first.
func branchMRExport(t *testing.T, b *Backend, kind, namespace, name, owner string, files ...git.FileEntry) error {
	t.Helper()
	ctx := git.WithCommitContext(context.Background(), git.CommitContext{Kind: kind, Namespace: namespace, Name: name, Cluster: "default"})
	err := b.ExportFiles(ctx, files, git.ExportFilesOptions{Prune: true, PruneOwner: owner})
	if err != nil && strings.Contains(err.Error(), "gitlab endpoint must use https or http") {
		return nil
	}
	return err
}

func newFileBranchMRBackend(remote string) *Backend {
	return &Backend{cfg: Config{
		Endpoint: "file://" + remote,
		MergeRequest: MergeRequestConfig{
			Mode:         MergeRequestModeBranchMR,
			TargetBranch: "main",
			BranchPrefix: "kollect",
		},
	}}
}

// TestBackend_BranchMR_clusterAndNamespacedInventoryUseSeparateBranches (IEI-11): KollectClusterInventory
// platform and KollectInventory cluster/platform render the same object path. Their merge requests
// must not share a feature branch, so neither MR carries the other's record, and no branch carries
// two claims on the one file both project.
func TestBackend_BranchMR_clusterAndNamespacedInventoryUseSeparateBranches(t *testing.T) {
	const shared = "inventory/cluster/platform.yaml"
	remote, _ := seedGitLabTestRemote(t)
	b := newFileBranchMRBackend(remote)
	l, _ := git.InventoryPruneOwner(kindClusterInventory, "default", "", "platform")
	n, _ := git.InventoryPruneOwner(kindNamespacedInventory, "default", "cluster", "platform")
	file := git.FileEntry{Path: shared, Data: []byte("kind: Inventory\n")}

	if err := branchMRExport(t, b, kindClusterInventory, "cluster", "platform", l, file); err != nil {
		t.Fatalf("cluster inventory export: %v", err)
	}
	if err := branchMRExport(t, b, kindNamespacedInventory, "cluster", "platform", n, file); err != nil {
		t.Fatalf("namespaced inventory export: %v", err)
	}

	branches := remoteBranches(t, remote, "kollect")
	if len(branches) != 2 {
		t.Fatalf("feature branches = %v, want one per inventory", branches)
	}
	for _, branch := range branches {
		owners := make([]string, 0, 2)
		for _, record := range branchRecords(t, remote, branch) {
			owners = append(owners, record.Owner)
		}
		if len(owners) != 1 {
			t.Fatalf("FORBIDDEN: branch %s carries the records of %v, want exactly one inventory's", branch, owners)
		}
	}
	assertNoBranchDuplicatesOnMerge(t, remote, "main", "kollect")
}

// TestBackend_BranchMR_refusesPathMergedForAnotherInventory (IEI-10): once one inventory's claim has
// merged into the target, another inventory's merge-request export of the same path is refused with
// a terminal error naming the owner, and pushes nothing that would merge into a duplicate claim.
func TestBackend_BranchMR_refusesPathMergedForAnotherInventory(t *testing.T) {
	const shared = "prod/team-a/Deployment/api.yaml"
	remote, runRemote := seedGitLabTestRemote(t)
	b := newFileBranchMRBackend(remote)
	a, _ := git.InventoryPruneOwner(kindNamespacedInventory, "default", "team-a", "apps")
	c, _ := git.InventoryPruneOwner(kindNamespacedInventory, "default", "team-c", "apps")
	file := git.FileEntry{Path: shared, Data: []byte("kind: Deployment\n")}

	if err := branchMRExport(t, b, kindNamespacedInventory, "team-a", "apps", a, file); err != nil {
		t.Fatalf("A's export: %v", err)
	}
	aBranch := BranchNameForExport("kollect", kindNamespacedInventory, "team-a", "apps")
	runRemote("update-ref", "refs/heads/main", "refs/heads/"+aBranch) // A's merge request merged

	err := branchMRExport(t, b, kindNamespacedInventory, "team-c", "apps", c, file)
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("C claims A's merged path: err = %v, want a terminal rejection", err)
	}
	for _, want := range []string{shared, "belongs to another inventory", `KollectInventory team-a/apps (cluster "default")`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("rejection %q does not name %q", err, want)
		}
	}
	cBranch := BranchNameForExport("kollect", kindNamespacedInventory, "team-c", "apps")
	if out, revErr := gitRemoteOutput(t, remote, "rev-parse", "--verify", "--quiet", "refs/heads/"+cBranch); revErr == nil {
		t.Fatalf("refused export pushed %s at %s", cBranch, out)
	}
	assertNoBranchDuplicatesOnMerge(t, remote, "main", "kollect")
}

// TestBranchNameForExport_kindSeparatesSamePathInventories (IEI-11): the two kinds that share an
// object path get different branches, a namespaced inventory keeps prefix/<namespace>/<name>, and a
// caller without a kind (deletion cleanup) gets the branch of the kind the path most likely names.
func TestBranchNameForExport_kindSeparatesSamePathInventories(t *testing.T) {
	t.Parallel()

	cluster := BranchNameForExport("kollect", kindClusterInventory, "cluster", "platform")
	namespaced := BranchNameForExport("kollect", kindNamespacedInventory, "cluster", "platform")
	if cluster == namespaced {
		t.Fatalf("FORBIDDEN: KollectClusterInventory platform and KollectInventory cluster/platform share branch %q", cluster)
	}
	if namespaced != "kollect/cluster/platform" {
		t.Fatalf("namespaced branch = %q, want kollect/cluster/platform", namespaced)
	}
	if cluster != "kollect/_cluster/platform" {
		t.Fatalf("cluster branch = %q, want kollect/_cluster/platform", cluster)
	}
	if got := BranchNameForExport("kollect", "", "cluster", "platform"); got != cluster {
		t.Fatalf("kind-less branch for inventory/cluster/platform = %q, want the cluster inventory's %q", got, cluster)
	}
	if got := BranchNameForExport("kollect", "", "team-a", "apps"); got != "kollect/team-a/apps" {
		t.Fatalf("kind-less branch for inventory/team-a/apps = %q", got)
	}
}
