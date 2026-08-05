// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/platformrelay/kollect/internal/collect"
)

type fakeExportCollection struct {
	replaceErr error
	deleteErr  error

	replaceCalls int
	deleteCalls  int
	lastFilter   bson.M
}

func (f *fakeExportCollection) ReplaceOne(
	_ context.Context,
	filter interface{},
	_ interface{},
	_ ...*options.ReplaceOptions,
) (*mongo.UpdateResult, error) {
	f.replaceCalls++
	if m, ok := filter.(bson.M); ok {
		f.lastFilter = m
	}
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	return &mongo.UpdateResult{MatchedCount: 1, UpsertedCount: 1}, nil
}

func (f *fakeExportCollection) DeleteMany(
	_ context.Context,
	filter interface{},
	_ ...*options.DeleteOptions,
) (*mongo.DeleteResult, error) {
	f.deleteCalls++
	if m, ok := filter.(bson.M); ok {
		f.lastFilter = m
	}
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return &mongo.DeleteResult{DeletedCount: 0}, nil
}

func testExportPayload(t *testing.T, items []collect.Item) []byte {
	t.Helper()
	payload, err := collect.MarshalExportEnvelope(items, collect.ExportMetadata{})
	if err != nil {
		t.Fatalf("MarshalExportEnvelope: %v", err)
	}
	return payload
}

func TestBackend_Export_upsertsAndReconcilesStale(t *testing.T) {
	t.Parallel()

	fake := &fakeExportCollection{}
	b := &Backend{
		cfg:  Config{Cluster: "prod-a"},
		coll: fake,
	}
	items := []collect.Item{{UID: "uid-1", TargetName: "deployments", Name: "api", Namespace: "workloads"}}
	payload := testExportPayload(t, items)

	if err := b.Export(context.Background(), payload, "inventory/team-a/apps.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if fake.replaceCalls != 1 || fake.deleteCalls != 1 {
		t.Fatalf("replace=%d delete=%d, want 1/1", fake.replaceCalls, fake.deleteCalls)
	}
	if got := fake.lastFilter["inventory_namespace"]; got != "team-a" {
		t.Fatalf("scope filter namespace = %v, want team-a", got)
	}
}

func TestBackend_Export_classifiesReplaceError(t *testing.T) {
	t.Parallel()

	fake := &fakeExportCollection{replaceErr: errors.New("connection reset")}
	b := &Backend{coll: fake}
	payload := testExportPayload(t, []collect.Item{{UID: "u1", TargetName: "pods"}})

	err := b.Export(context.Background(), payload, "inventory/ns/inv.json")
	if err == nil {
		t.Fatal("expected replace error")
	}
	if !errors.Is(err, ErrUpsertFailed) {
		t.Fatalf("error = %v, want ErrUpsertFailed", err)
	}
}

func TestBackend_Close_disconnectsClient(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	b := &Backend{client: client}
	b.Close()
	if err := client.Disconnect(context.Background()); err == nil {
		// second disconnect on already disconnected client is fine; ensure not panicking
	}
}

func TestBackend_Export_deleteStaleError(t *testing.T) {
	t.Parallel()

	fake := &fakeExportCollection{deleteErr: errors.New("delete boom")}
	b := &Backend{coll: fake}
	payload := testExportPayload(t, []collect.Item{{UID: "u1", TargetName: "pods"}})

	err := b.Export(context.Background(), payload, "inventory/ns/inv.json")
	if err == nil || !strings.Contains(err.Error(), "mongodb delete stale") {
		t.Fatalf("error = %q, want delete stale failure", err)
	}
}

func TestEnsureCollection_recordsUniqueIndexKeys(t *testing.T) {
	t.Parallel()

	fa := &fakeCollectionAdmin{names: []string{"items"}}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	if err := b.ensureCollection(context.Background()); err != nil {
		t.Fatalf("ensureCollection: %v", err)
	}
	if fa.indexCalls != 1 || len(fa.lastKeys) != 4 {
		t.Fatalf("index keys = %#v, want four compound keys", fa.lastKeys)
	}
}
func TestBackend_Export_emptySnapshotDeletesScope(t *testing.T) {
	t.Parallel()

	fake := &fakeExportCollection{}
	b := &Backend{coll: fake, cfg: Config{Cluster: "prod-a"}}
	payload := testExportPayload(t, nil)

	if err := b.Export(context.Background(), payload, "inventory/team-a/apps.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if fake.replaceCalls != 0 || fake.deleteCalls != 1 {
		t.Fatalf("replace=%d delete=%d, want 0/1", fake.replaceCalls, fake.deleteCalls)
	}
}
func TestBackend_Export_terminalDuplicateKey(t *testing.T) {
	t.Parallel()

	dup := mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: mongoCodeDuplicateKey, Message: "dup"}}}
	fake := &fakeExportCollection{replaceErr: dup}
	b := &Backend{coll: fake}
	payload := testExportPayload(t, []collect.Item{{UID: "u1", TargetName: "pods"}})

	err := b.Export(context.Background(), payload, "inventory/ns/inv.json")
	if err == nil || !errors.Is(err, ErrUpsertFailed) {
		t.Fatalf("error = %v, want classified duplicate key", err)
	}
}
