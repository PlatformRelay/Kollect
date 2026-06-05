// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"encoding/json"
	"testing"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/hub"
)

func TestValidateClusterWire(t *testing.T) {
	t.Parallel()

	report := hub.SpokeReport{Cluster: "spoke-a"}

	if err := hub.ValidateClusterWire("", report); err != nil {
		t.Fatalf("empty wire: %v", err)
	}

	if err := hub.ValidateClusterWire("spoke-a", report); err != nil {
		t.Fatalf("match: %v", err)
	}

	if err := hub.ValidateClusterWire("other", report); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestReceiveReportMergesWithWireCluster(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := hub.NewMerger(store)

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: hub.InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "t",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-1",
			Version:         "v1",
			Kind:            "Deployment",
		}},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	got, applied, err := hub.ReceiveReport("spoke-a", payload, merger, nil, false)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if applied != 1 {
		t.Fatalf("applied = %d", applied)
	}

	if got.Cluster != "spoke-a" {
		t.Fatalf("cluster = %q", got.Cluster)
	}

	snap := store.SnapshotNamespace("spoke-a")
	if len(snap) != 1 {
		t.Fatalf("snapshot len = %d", len(snap))
	}
}

func TestReceiveReportRejectsUnregisteredCluster(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := hub.NewMerger(store)

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "rogue",
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = hub.ReceiveReport("rogue", payload, merger, []string{"spoke-a"}, true)
	if err == nil {
		t.Fatal("expected acl error")
	}

	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0", store.TotalCount())
	}
}

func TestReceiveReportDefaultsLegacySchemaVersion(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := hub.NewMerger(store)

	payload := []byte(`{
		"apiVersion":"kollect.dev/v1alpha1",
		"cluster":"spoke-a",
		"inventoryRef":{"namespace":"team-a","name":"inv"},
		"items":[{"namespace":"apps","name":"demo","uid":"uid-1","version":"v1","kind":"Deployment"}]
	}`)

	got, applied, err := hub.ReceiveReport("spoke-a", payload, merger, nil, false)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if applied != 1 {
		t.Fatalf("applied = %d", applied)
	}

	if got.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", got.SchemaVersion, export.SchemaVersion)
	}
}

func TestReceiveReportRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	merger := hub.NewMerger(store)

	report := hub.SpokeReport{
		APIVersion:    export.WireAPIVersion,
		SchemaVersion: "kollect.dev/v99",
		Cluster:       "spoke-a",
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = hub.ReceiveReport("spoke-a", payload, merger, nil, false)
	if err == nil {
		t.Fatal("expected schemaVersion error")
	}

	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0", store.TotalCount())
	}
}

func TestReceiveReportNilMerger(t *testing.T) {
	t.Parallel()

	_, _, err := hub.ReceiveReport("spoke-a", []byte(`{}`), nil, nil, false)
	if err == nil {
		t.Fatal("expected error for nil merger")
	}
}

func TestReceiveReportInvalidJSON(t *testing.T) {
	t.Parallel()

	merger := hub.NewMerger(collect.NewStore())
	_, _, err := hub.ReceiveReport("", []byte("{"), merger, nil, false)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestReceiveReportEmptyCluster(t *testing.T) {
	t.Parallel()

	merger := hub.NewMerger(collect.NewStore())
	_, _, err := hub.ReceiveReport("", []byte(`{"cluster":""}`), merger, nil, false)
	if err == nil {
		t.Fatal("expected missing cluster error")
	}
}

func TestValidateClusterWireEmptyBodyCluster(t *testing.T) {
	t.Parallel()

	if err := hub.ValidateClusterWire("spoke-a", hub.SpokeReport{}); err == nil {
		t.Fatal("expected error when report cluster is empty")
	}
}

func TestReceiveReportFillsClusterFromWireHeader(t *testing.T) {
	t.Parallel()

	merger := hub.NewMerger(collect.NewStore())
	payload := []byte(`{
		"apiVersion":"kollect.dev/v1alpha1",
		"inventoryRef":{"namespace":"team-a","name":"inv"},
		"items":[{"namespace":"apps","name":"demo","uid":"uid-1","version":"v1","kind":"Deployment"}]
	}`)

	got, applied, err := hub.ReceiveReport("spoke-a", payload, merger, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if applied != 1 || got.Cluster != "spoke-a" {
		t.Fatalf("got = %+v applied = %d", got, applied)
	}
}
