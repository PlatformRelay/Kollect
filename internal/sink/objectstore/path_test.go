// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package objectstore

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestObjectPath(t *testing.T) {
	t.Parallel()

	jsonPath := ObjectPath(kollectdevv1alpha1.KollectSinkSpec{Type: "s3"}, "team-a", "deployments", 7)
	if jsonPath != "inventory/team-a/deployments.json" {
		t.Fatalf("json path = %q", jsonPath)
	}

	parquetPath := ObjectPath(kollectdevv1alpha1.KollectSinkSpec{
		Type:    "s3",
		Cluster: "prod-west",
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{
			Format: FormatParquet,
		},
	}, "team-a", "deployments", 7)
	want := "inventory/cluster=prod-west/ns=team-a/name=deployments/generation=7.parquet"
	if parquetPath != want {
		t.Fatalf("parquet path = %q, want %q", parquetPath, want)
	}
}

func TestInventoryFromObjectPath(t *testing.T) {
	t.Parallel()

	ns, name := InventoryFromObjectPath("inventory/team-a/deployments.json")
	if ns != "team-a" || name != "deployments" {
		t.Fatalf("got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("inventory/cluster=prod/ns=team-a/name=deployments/generation=3.parquet")
	if ns != "team-a" || name != "deployments" {
		t.Fatalf("parquet got (%q, %q)", ns, name)
	}

	ns, name = InventoryFromObjectPath("inventory/cluster/rollup.json")
	if ns != "cluster" || name != "rollup" {
		t.Fatalf("cluster rollup got (%q, %q)", ns, name)
	}
}
