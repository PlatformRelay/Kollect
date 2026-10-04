// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	billy "github.com/go-git/go-billy/v5"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// TestInventoryPruneOwner_golden pins the kind-qualified owner encoding (IEI-1, IEI-10): the bytes
// and the record path every inventory's ownership record lives under.
func TestInventoryPruneOwner_golden(t *testing.T) {
	for _, tc := range []struct {
		kind, cluster, namespace, name string
		want                           string
	}{
		{"KollectClusterInventory", "default", "", "platform", `["v2","KollectClusterInventory","default","","platform"]`},
		{"KollectInventory", "default", "cluster", "platform", `["v2","KollectInventory","default","cluster","platform"]`},
		{"KollectInventory", "prod", "team-a", "apps", `["v2","KollectInventory","prod","team-a","apps"]`},
	} {
		got, err := InventoryPruneOwner(tc.kind, tc.cluster, tc.namespace, tc.name)
		if err != nil {
			t.Fatalf("InventoryPruneOwner(%q, %q, %q, %q): %v", tc.kind, tc.cluster, tc.namespace, tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("owner = %s, want %s", got, tc.want)
		}
		wantRecord := fmt.Sprintf(".kollect-prune/%x.json", sha256.Sum256([]byte(tc.want)))
		if rp := pruneRecordPath(got); rp != wantRecord {
			t.Fatalf("record path = %s, want %s", rp, wantRecord)
		}
	}
	cluster, _ := InventoryPruneOwner("KollectClusterInventory", "default", "", "platform")
	namespaced, _ := InventoryPruneOwner("KollectInventory", "default", "cluster", "platform")
	if cluster == namespaced || pruneRecordPath(cluster) == pruneRecordPath(namespaced) {
		t.Fatal("cluster inventory and namespaced inventory in namespace cluster share an owner")
	}
	if _, err := InventoryPruneOwner("", "default", "team-a", "apps"); err == nil {
		t.Fatal("owner without a kind accepted")
	}
}

// TestOwnedPrune_rejectionNamesOwningInventory: the "belongs to another inventory" rejection names
// the owner (kind, namespace/name) and the record file that claims the path.
func TestOwnedPrune_rejectionNamesOwningInventory(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		owner, _ := InventoryPruneOwner("KollectInventory", "default", "cluster", "platform")
		other, _ := InventoryPruneOwner("KollectClusterInventory", "default", "", "platform")
		commitOwnedPaths(t, fs, owner, "default/team-a/deployment/api.yaml")

		_, err := prepareOwnedPrune(fs, Config{Prune: true, PruneOwner: other}, []string{"default/team-a/deployment/api.yaml"})
		if !kollecterrors.IsTerminal(err) {
			t.Fatalf("claim of another owner's path: err = %v, want terminal", err)
		}
		for _, want := range []string{
			"belongs to another inventory",
			`KollectInventory cluster/platform (cluster "default")`,
			pruneRecordPath(owner),
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("rejection %q does not name %q", err, want)
			}
		}
	})
}

// TestOwnedPrune_claimPathsCheckedAgainstOtherOwners: a claim path (the set manifest a later part
// writes) owned by another inventory rejects the call before anything is written or recorded.
func TestOwnedPrune_claimPathsCheckedAgainstOtherOwners(t *testing.T) {
	ownedFilesystems(t, func(t *testing.T, fs billy.Filesystem) {
		const manifest = "inventory/cluster/platform.manifest.json"
		owner, _ := InventoryPruneOwner("KollectInventory", "default", "cluster", "platform")
		claimer, _ := InventoryPruneOwner("KollectClusterInventory", "default", "", "platform")
		commitOwnedPaths(t, fs, owner, manifest)

		cfg := Config{PruneOwner: claimer, PruneClaimPaths: []string{manifest}}
		_, err := prepareOwnedPrune(fs, cfg, []string{"default/clusterrole/admin.yaml"})
		if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), "belongs to another inventory") ||
			!strings.Contains(err.Error(), manifest) {
			t.Fatalf("foreign claim path: err = %v, want a terminal ownership rejection naming %q", err, manifest)
		}

		// The owner itself may pre-claim its own manifest.
		if _, err := prepareOwnedPrune(fs, Config{PruneOwner: owner, PruneClaimPaths: []string{manifest}}, []string{"x/y.yaml"}); err != nil {
			t.Fatalf("own claim path rejected: %v", err)
		}
	})
}

// TestCheckSingleClaims_rejectsDuplicate: the records a commit would leave behind may claim each
// path once. A collision with the exporting owner reads as that other inventory's path; any other
// collision names the path and both owners.
func TestCheckSingleClaims_rejectsDuplicate(t *testing.T) {
	records := map[string]pruneRecord{
		"a": {Version: 1, Owner: "a", Paths: []string{"x/one.yaml", "x/shared.yaml"}},
		"b": {Version: 1, Owner: "b", Paths: []string{"x/shared.yaml", "x/two.yaml"}},
	}
	err := checkSingleClaims(records, "c")
	if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), `would claim "x/shared.yaml" twice`) {
		t.Fatalf("duplicate claim: err = %v, want terminal error naming the path", err)
	}
	err = checkSingleClaims(records, "b")
	if !kollecterrors.IsTerminal(err) || !strings.Contains(err.Error(), `"x/shared.yaml" belongs to another inventory`) ||
		!strings.Contains(err.Error(), pruneRecordPath("a")) {
		t.Fatalf("claim by the exporting owner: err = %v, want it named as owner a's path", err)
	}
	delete(records, "b")
	if err := checkSingleClaims(records, "a"); err != nil {
		t.Fatalf("single claims rejected: %v", err)
	}
	if err := checkSingleClaims(map[string]pruneRecord{"a": {Version: 1, Owner: "a", Paths: []string{"x/d.yaml", "x/d.yaml"}}}, "a"); err == nil {
		t.Fatal("a record listing one path twice accepted")
	}
}
