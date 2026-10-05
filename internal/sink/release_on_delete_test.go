// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// recordPathOf is the ownership record path of id on a sink without spec.cluster.
func recordPathOf(t *testing.T, id InventoryIdentity) string {
	t.Helper()
	owner, err := git.InventoryPruneOwner(id.Kind, "default", id.Namespace, id.Name)
	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf(".kollect-prune/%x.json", sha256.Sum256([]byte(owner)))
}

// cleanup runs the inventory-deletion cleanup of id against the remote with the given policy.
func (r *identityRemote) cleanup(id InventoryIdentity, policy string, shared bool) (CleanupExportOutcome, error) {
	r.t.Helper()
	ResetBreakersForTest()
	spec := r.spec
	spec.DeletionPolicy = policy

	return RunCleanupExport(CleanupExportRequest{
		Ctx:                  r.t.Context(),
		Registry:             NewRegistry(),
		SinkNamespace:        "default",
		SinkName:             "identity-git",
		SinkUID:              "uid-identity-git",
		SinkSpec:             spec,
		ObjectPath:           platformObjectPath,
		Inventory:            id,
		Generation:           1,
		SharedExportIdentity: shared,
	})
}

// changedPaths lists paths whose presence or bytes differ between two snapshots.
func changedPaths(before, after map[string]string) []string {
	var out []string
	for p, v := range before {
		if w, ok := after[p]; !ok || w != v {
			out = append(out, p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)

	return out
}

const (
	lResource = "default/clusterrole/admin.yaml"
	nResource = "default/cluster/configmap/settings.yaml"
)

// seedLAndN exports disjoint trees for L (KollectClusterInventory platform) and N (KollectInventory
// cluster/platform), which share object path inventory/cluster/platform.json.
func seedLAndN(t *testing.T, r *identityRemote) {
	t.Helper()
	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{identityItem("", "ClusterRole", "admin", "L")}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.export(namespacedPlatform, platformObjectPath, []collect.Item{identityItem("cluster", "ConfigMap", "settings", "N")}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
}

// TestRunCleanupExport_retainReleasesRecordKeepsFiles (ROD-1, ROD-2, ROD-3): a Retain deletion of
// either inventory removes exactly its own record; every exported file and the other inventory's
// record keep their bytes.
func TestRunCleanupExport_retainReleasesRecordKeepsFiles(t *testing.T) {
	for _, deleting := range []InventoryIdentity{clusterPlatform, namespacedPlatform} {
		t.Run(deleting.Kind, func(t *testing.T) {
			r := newIdentityRemote(t)
			seedLAndN(t, r)
			before := r.files()
			own := recordPathOf(t, deleting)
			if _, ok := before[own]; !ok {
				t.Fatalf("seed wrote no record %s for %s", own, deleting)
			}

			outcome, err := r.cleanup(deleting, kollectdevv1alpha1.DeletionPolicyRetain, false)
			if err != nil {
				t.Fatalf("Retain cleanup: %v", err)
			}
			if outcome != CleanupRetainedByPolicy {
				t.Fatalf("outcome = %v, want CleanupRetainedByPolicy", outcome)
			}
			if got := changedPaths(before, r.files()); len(got) != 1 || got[0] != own {
				t.Fatalf("Retain deletion of %s changed %v, want exactly its record %s "+
					"(FORBIDDEN: the record survives / Retain deletes a file / another record is touched)", deleting, got, own)
			}
		})
	}
}

// TestRunCleanupExport_deleteRetractsOwnTreeAndRecord (ROD-1, ROD-3, ROD-4): a Delete deletion of N
// removes N's tree and record; L's tree and record, on the same object path, are kept.
func TestRunCleanupExport_deleteRetractsOwnTreeAndRecord(t *testing.T) {
	r := newIdentityRemote(t)
	seedLAndN(t, r)
	before := r.files()

	if _, err := r.cleanup(namespacedPlatform, kollectdevv1alpha1.DeletionPolicyDelete, false); err != nil {
		t.Fatalf("Delete cleanup: %v", err)
	}
	want := []string{nResource, recordPathOf(t, namespacedPlatform)}
	sort.Strings(want)
	if got := changedPaths(before, r.files()); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Delete deletion of N changed %v, want exactly %v (FORBIDDEN: L's record or files touched)", got, want)
	}
}

// TestRunCleanupExport_sharedIdentityReleasesRecordOnly (ROD-1, ROD-3): with Delete but a live
// inventory of the same export identity, the retraction is skipped and only the record is released.
func TestRunCleanupExport_sharedIdentityReleasesRecordOnly(t *testing.T) {
	r := newIdentityRemote(t)
	seedLAndN(t, r)
	before := r.files()

	outcome, err := r.cleanup(clusterPlatform, kollectdevv1alpha1.DeletionPolicyDelete, true)
	if err != nil {
		t.Fatalf("shared-identity cleanup: %v", err)
	}
	if outcome != CleanupRetainedSharedIdentity {
		t.Fatalf("outcome = %v, want CleanupRetainedSharedIdentity", outcome)
	}
	if got := changedPaths(before, r.files()); len(got) != 1 || got[0] != recordPathOf(t, clusterPlatform) {
		t.Fatalf("shared-identity deletion changed %v, want only L's record", got)
	}
}

// TestRunCleanupExport_successorTakesOverReleasedPaths (ROD-7): after L's Retain deletion, N may
// export the path L's record listed; before the release it was refused.
func TestRunCleanupExport_successorTakesOverReleasedPaths(t *testing.T) {
	r := newIdentityRemote(t)
	shared := identityItem("team-a", "Deployment", "api", "L")
	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{shared}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.cleanup(clusterPlatform, "", false); err != nil {
		t.Fatalf("Retain cleanup: %v", err)
	}
	if err := r.export(namespacedPlatform, platformObjectPath, []collect.Item{identityItem("team-a", "Deployment", "api", "successor")}, 1, 1, nil); err != nil {
		t.Fatalf("successor export of a released path: %v", err)
	}
	files := r.files()
	if !strings.Contains(files["default/team-a/deployment/api.yaml"], "kollect.dev/test-writer: successor") {
		t.Fatalf("successor did not take over the released path: %v", files)
	}
	if !strings.Contains(files[recordPathOf(t, namespacedPlatform)], "default/team-a/deployment/api.yaml") {
		t.Fatal("successor's record does not list the taken-over path")
	}
}

// TestRunCleanupExport_recreateStartsNewRecord (IEI-8): a recreated inventory with the same identity
// starts a new record after a Retain deletion and deletes none of its predecessor's files.
func TestRunCleanupExport_recreateStartsNewRecord(t *testing.T) {
	r := newIdentityRemote(t)
	api := identityItem("team-a", "Deployment", "api", "L")
	web := identityItem("team-a", "Deployment", "web", "L")
	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{api, web}, 1, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.cleanup(clusterPlatform, "", false); err != nil {
		t.Fatalf("Retain cleanup: %v", err)
	}
	if err := r.export(clusterPlatform, platformObjectPath, []collect.Item{api}, 1, 1, nil); err != nil {
		t.Fatalf("recreated export: %v", err)
	}
	files := r.files()
	requirePaths(t, files, "default/team-a/deployment/api.yaml", "default/team-a/deployment/web.yaml")
	if strings.Contains(files[recordPathOf(t, clusterPlatform)], "web.yaml") {
		t.Fatalf("recreated inventory continued its predecessor's record:\n%s", files[recordPathOf(t, clusterPlatform)])
	}
}

// TestRunCleanupExport_gitReleaseFailureIsRetried (ROD-6): a git sink whose backend cannot be built
// or reached fails the Retain cleanup with a transient error, so the finalizer stays and retries; it
// is never reported as retained by policy.
func TestRunCleanupExport_gitReleaseFailureIsRetried(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() { EnableBackendPoolForTest(); ResetBackendPoolForTest(); ResetBreakersForTest() })

	for _, sinkType := range []string{kollectdevv1alpha1.SnapshotSinkTypeGit, kollectdevv1alpha1.SnapshotSinkTypeGitLab} {
		t.Run(sinkType+"/build fails", func(t *testing.T) {
			built := 0
			outcome, err := RunCleanupExport(CleanupExportRequest{
				Ctx: t.Context(), Registry: countingRegistry(sinkType, nil, &built),
				SinkName: "unreachable", SinkUID: "uid-unreachable",
				SinkSpec:   kollectdevv1alpha1.KollectSinkSpec{Type: sinkType},
				ObjectPath: platformObjectPath, Inventory: clusterPlatform,
			})
			if err == nil || kollecterrors.IsTerminal(err) {
				t.Fatalf("Retain release with an unbuildable backend: outcome %v, err = %v; want a transient error", outcome, err)
			}
			if built != 1 {
				t.Fatalf("backend built %d times, want 1 (the release must be attempted)", built)
			}
		})
	}

	t.Run("git/remote unreachable", func(t *testing.T) {
		r := newIdentityRemote(t)
		r.spec.Endpoint = "file://" + r.work + "/missing.git"
		_, err := r.cleanup(clusterPlatform, "", false)
		if err == nil {
			t.Fatal("Retain release against a missing remote succeeded; the record cannot have been released")
		}
		if kollecterrors.IsTerminal(err) {
			t.Fatalf("Retain release against a missing remote is terminal (%v); an unreachable backend must stay transient so the finalizer retries", err)
		}
	})
}

// TestRunCleanupExport_gitReleaseNeedsIdentity (ROD-1): a git sink cleanup without an identity is
// terminal before the backend is built.
func TestRunCleanupExport_gitReleaseNeedsIdentity(t *testing.T) {
	built := 0
	_, err := RunCleanupExport(CleanupExportRequest{
		Ctx: t.Context(), Registry: countingRegistry(kollectdevv1alpha1.SnapshotSinkTypeGit, nil, &built),
		SinkName: "no-identity", SinkUID: "uid-no-identity",
		SinkSpec:   kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit},
		ObjectPath: platformObjectPath,
	})
	if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), "inventory identity") {
		t.Fatalf("identity-less git cleanup: err = %v, want a terminal identity error", err)
	}
	if built != 0 {
		t.Fatalf("backend built %d times without an identity", built)
	}
}

// TestRunCleanupExport_releaseUsesSinkCluster (ROD-1, ROD-3): the cleanup derives the owner with the
// sink's spec.cluster, as the export does. The same inventory exported through a "prod" sink and a
// default-cluster sink of one repository has two records; deleting it through the prod sink releases
// only the prod record and, under Delete, removes only the prod files.
func TestRunCleanupExport_releaseUsesSinkCluster(t *testing.T) {
	for _, policy := range []string{kollectdevv1alpha1.DeletionPolicyRetain, kollectdevv1alpha1.DeletionPolicyDelete} {
		t.Run(policy, func(t *testing.T) {
			r := newIdentityRemote(t)
			item := []collect.Item{identityItem("", "ClusterRole", "admin", "L")}
			if err := r.export(clusterPlatform, platformObjectPath, item, 1, 1, nil); err != nil {
				t.Fatal(err)
			}
			r.spec.Cluster = "prod"
			if err := r.export(clusterPlatform, platformObjectPath, item, 1, 1, nil); err != nil {
				t.Fatal(err)
			}
			prodOwner, err := git.InventoryPruneOwner(clusterPlatform.Kind, "prod", "", clusterPlatform.Name)
			if err != nil {
				t.Fatal(err)
			}
			prodRecord := fmt.Sprintf(".kollect-prune/%x.json", sha256.Sum256([]byte(prodOwner)))
			defaultRecord := recordPathOf(t, clusterPlatform)
			before := r.files()
			for _, p := range []string{prodRecord, defaultRecord, "prod/clusterrole/admin.yaml", lResource} {
				if _, ok := before[p]; !ok {
					t.Fatalf("seed is missing %s; files %v", p, dataPaths(before))
				}
			}

			if _, err := r.cleanup(clusterPlatform, policy, false); err != nil {
				t.Fatalf("%s cleanup through the prod sink: %v", policy, err)
			}
			want := []string{prodRecord}
			if policy == kollectdevv1alpha1.DeletionPolicyDelete {
				want = append(want, "prod/clusterrole/admin.yaml")
			}
			sort.Strings(want)
			if got := changedPaths(before, r.files()); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("%s deletion through the prod sink changed %v, want exactly %v "+
					"(FORBIDDEN: the default-cluster owner's record or files touched)", policy, got, want)
			}
		})
	}
}
