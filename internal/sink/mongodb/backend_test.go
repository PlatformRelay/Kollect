// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

func TestBackend_TypeAndCapabilities(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q, want %q", b.Type(), TypeName)
	}

	caps := b.Capabilities()
	if !caps.SupportsDelete {
		t.Fatal("mongodb must support delete reconciliation")
	}
	if caps.ObjectStore {
		t.Fatal("mongodb must not be an object-store")
	}
	if caps.Stream {
		t.Fatal("mongodb must not be a stream emitter")
	}
}

func TestBackend_Close_nilClientIsNoop(t *testing.T) {
	t.Parallel()

	b := &Backend{client: nil}
	b.Close() // must not panic
}

func TestBackend_Export_decodeError(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	err := b.Export(context.Background(), []byte(`{"schemaVersion":"kollect.dev/v99","items":[]}`), "")
	if err == nil {
		t.Fatal("expected decode error for unsupported schema version")
	}
	if !strings.Contains(err.Error(), "decode payload") {
		t.Fatalf("error = %q, want decode payload context", err)
	}
}

func TestNewBackend_configError(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(context.Background(), kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"}, nil)
	if err == nil {
		t.Fatal("expected config error for wrong sink type")
	}
}

func TestNewBackend_unreachableHost(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}

	_, err := NewBackend(ctx, spec, map[string][]byte{"uri": []byte("mongodb://127.0.0.1:1/unreachable")})
	if err == nil {
		t.Fatal("expected connect error for unreachable host")
	}
	if !strings.Contains(err.Error(), "mongodb") {
		t.Fatalf("error = %q, want mongodb context", err)
	}
}

type fakeDeleteManyCollection struct {
	gotFilter any
	err       error
}

func (f *fakeDeleteManyCollection) DeleteMany(
	_ context.Context,
	filter interface{},
	_ ...*options.DeleteOptions,
) (*mongo.DeleteResult, error) {
	f.gotFilter = filter
	if f.err != nil {
		return nil, f.err
	}

	return &mongo.DeleteResult{DeletedCount: 1}, nil
}

func TestItemDocument_UsesInventoryNamespaceFallback(t *testing.T) {
	t.Parallel()

	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	ts := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
	doc, err := itemDocument(
		scope,
		collect.Item{
			UID:        "uid-1",
			TargetName: "deployments",
			Namespace:  "",
			Name:       "api",
		},
		ts,
	)
	if err != nil {
		t.Fatalf("itemDocument: %v", err)
	}

	if got := doc["resource_namespace"]; got != "team-a" {
		t.Fatalf("resource_namespace = %v, want team-a", got)
	}
	if got := doc["target_name"]; got != "deployments" {
		t.Fatalf("target_name = %v, want deployments", got)
	}
	payload, ok := doc["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", doc["payload"])
	}
	if got := payload["name"]; got != "api" {
		t.Fatalf("payload.name = %v, want api", got)
	}
}

func TestItemDocument_UsesItemNamespaceWhenSet(t *testing.T) {
	t.Parallel()

	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	ts := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
	doc, err := itemDocument(
		scope,
		collect.Item{
			UID:        "uid-1",
			TargetName: "deployments",
			Namespace:  "workloads",
			Name:       "api",
		},
		ts,
	)
	if err != nil {
		t.Fatalf("itemDocument: %v", err)
	}

	if got := doc["resource_namespace"]; got != "workloads" {
		t.Fatalf("resource_namespace = %v, want workloads", got)
	}
	if got := doc["cluster"]; got != "prod-a" {
		t.Fatalf("cluster = %v, want prod-a", got)
	}
	if got := doc["exported_at"]; !got.(time.Time).Equal(ts) {
		t.Fatalf("exported_at = %v, want %v", got, ts)
	}
}

func TestItemDocument_ReportsMarshalError(t *testing.T) {
	t.Parallel()

	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	_, err := itemDocument(
		scope,
		collect.Item{
			UID:        "uid-1",
			TargetName: "deployments",
			Attributes: map[string]any{"ratio": math.NaN()},
		},
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected marshal error for NaN payload")
	}
}

func TestStaleDeleteFilter_DeleteAllScopeOnly(t *testing.T) {
	t.Parallel()

	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	filter, deleteAll := staleDeleteFilter(scope, nil)
	if !deleteAll {
		t.Fatal("deleteAll = false, want true")
	}
	if _, hasNor := filter["$nor"]; hasNor {
		t.Fatalf("filter unexpectedly contains $nor: %#v", filter)
	}
	if got := filter["inventory_namespace"]; got != "team-a" {
		t.Fatalf("inventory_namespace = %v, want team-a", got)
	}
}

func TestStaleDeleteFilter_ExcludesCurrentSnapshotItems(t *testing.T) {
	t.Parallel()

	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	items := []collect.Item{
		{TargetName: "deployments", UID: "uid-1"},
		{TargetName: "pods", UID: "uid-2"},
	}
	filter, deleteAll := staleDeleteFilter(scope, items)
	if deleteAll {
		t.Fatal("deleteAll = true, want false")
	}

	nor, ok := filter["$nor"].([]bson.M)
	if !ok {
		t.Fatalf("$nor type = %T, want []bson.M", filter["$nor"])
	}
	if len(nor) != 2 {
		t.Fatalf("$nor len = %d, want 2", len(nor))
	}
	if nor[0]["target_name"] != "deployments" || nor[0]["source_uid"] != "uid-1" {
		t.Fatalf("unexpected first $nor filter: %#v", nor[0])
	}
}

func TestNewExportScopeAndUpsertFilter(t *testing.T) {
	t.Parallel()

	scope := newExportScope("inventory/team-a/apps.json", "prod-a")
	if scope.inventoryNamespace != "team-a" || scope.inventoryName != "apps" || scope.cluster != "prod-a" {
		t.Fatalf("scope = %#v", scope)
	}

	filter := upsertFilter(scope, collect.Item{TargetName: "deployments", UID: "uid-1"})
	if got := filter["inventory_namespace"]; got != "team-a" {
		t.Fatalf("inventory_namespace = %v, want team-a", got)
	}
	if got := filter["target_name"]; got != "deployments" {
		t.Fatalf("target_name = %v, want deployments", got)
	}
	if got := filter["source_uid"]; got != "uid-1" {
		t.Fatalf("source_uid = %v, want uid-1", got)
	}
}

func TestDeleteStaleDocuments_ReportsDeleteAllFailure(t *testing.T) {
	t.Parallel()

	fake := &fakeDeleteManyCollection{err: errors.New("boom")}
	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}

	err := deleteStaleDocuments(context.Background(), fake, scope, nil)
	if err == nil {
		t.Fatal("deleteStaleDocuments returned nil error, want failure")
	}
	if !strings.Contains(err.Error(), "mongodb delete all: boom") {
		t.Fatalf("error = %q, want mongodb delete all context", err)
	}
}

func TestDeleteStaleDocuments_ReportsDeleteStaleFailure(t *testing.T) {
	t.Parallel()

	fake := &fakeDeleteManyCollection{err: errors.New("boom")}
	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	items := []collect.Item{{TargetName: "pods", UID: "uid-1"}}

	err := deleteStaleDocuments(context.Background(), fake, scope, items)
	if err == nil {
		t.Fatal("deleteStaleDocuments returned nil error, want failure")
	}
	if !strings.Contains(err.Error(), "mongodb delete stale: boom") {
		t.Fatalf("error = %q, want mongodb delete stale context", err)
	}
}

func TestDeleteStaleDocuments_PassesScopeAndNorFilter(t *testing.T) {
	t.Parallel()

	fake := &fakeDeleteManyCollection{}
	scope := exportScope{
		inventoryNamespace: "team-a",
		inventoryName:      "apps",
		cluster:            "prod-a",
	}
	items := []collect.Item{{TargetName: "deployments", UID: "uid-1"}}

	if err := deleteStaleDocuments(context.Background(), fake, scope, items); err != nil {
		t.Fatalf("deleteStaleDocuments: %v", err)
	}

	filter, ok := fake.gotFilter.(bson.M)
	if !ok {
		t.Fatalf("DeleteMany filter type = %T, want bson.M", fake.gotFilter)
	}
	if got := filter["inventory_namespace"]; got != "team-a" {
		t.Fatalf("inventory_namespace = %v, want team-a", got)
	}
	nor, ok := filter["$nor"].([]bson.M)
	if !ok || len(nor) != 1 {
		t.Fatalf("$nor = %#v, want single delete guard", filter["$nor"])
	}
}

// inventoryFromObjectPath behavior is now centrally tested in
// internal/pathvalidate (TestInventoryFromObjectPath); TestNewExportScopeAndUpsertFilter
// above exercises the wiring through newExportScope.
