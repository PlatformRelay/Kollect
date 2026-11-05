// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/xoptions"
)

// mockAdmin returns a mongoCollectionAdmin over a client whose deployment answers
// each command with the next of responses (OP_MSG), so the real driver code path of
// the admin methods runs without a server. mongo-driver v2 moved mtest under
// internal/; drivertest.MockDeployment is the public equivalent of its mock client.
func mockAdmin(t *testing.T, responses ...bson.D) mongoCollectionAdmin {
	t.Helper()

	opts := options.Client()
	if err := xoptions.SetInternalClientOptions(opts, "deployment", drivertest.NewMockDeployment(responses...)); err != nil {
		t.Fatalf("set mock deployment: %v", err)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	db := client.Database("db")

	return mongoCollectionAdmin{db: db, coll: db.Collection("items")}
}

func cursorResponse(ns string, batch ...bson.D) bson.D {
	docs := make(bson.A, 0, len(batch))
	for _, doc := range batch {
		docs = append(docs, doc)
	}

	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: int64(0)},
			{Key: "ns", Value: ns},
			{Key: "firstBatch", Value: docs},
		}},
	}
}

func okResponse() bson.D {
	return bson.D{{Key: "ok", Value: 1}}
}

func TestMongoCollectionAdmin_listCreateIndex(t *testing.T) {
	t.Parallel()

	t.Run("list names", func(t *testing.T) {
		t.Parallel()

		admin := mockAdmin(t, cursorResponse("db.$cmd.listCollections", bson.D{{Key: "name", Value: "items"}}))
		names, err := admin.ListCollectionNames(context.Background(), bson.M{"name": "items"})
		if err != nil {
			t.Fatalf("ListCollectionNames: %v", err)
		}
		if len(names) != 1 || names[0] != "items" {
			t.Fatalf("names = %v, want [items]", names)
		}
	})

	t.Run("create collection", func(t *testing.T) {
		t.Parallel()

		admin := mockAdmin(t, okResponse())
		if err := admin.CreateCollection(context.Background(), "items"); err != nil {
			t.Fatalf("CreateCollection: %v", err)
		}
	})

	t.Run("ensure index", func(t *testing.T) {
		t.Parallel()

		admin := mockAdmin(t, okResponse())
		if err := admin.EnsureUniqueIndex(context.Background(), bson.D{{Key: "source_uid", Value: 1}}); err != nil {
			t.Fatalf("EnsureUniqueIndex: %v", err)
		}
	})

	t.Run("server error surfaces", func(t *testing.T) {
		t.Parallel()

		admin := mockAdmin(t, bson.D{
			{Key: "ok", Value: 0},
			{Key: "code", Value: 13},
			{Key: "errmsg", Value: "not authorized"},
			{Key: "codeName", Value: "Unauthorized"},
		})
		if err := admin.CreateCollection(context.Background(), "items"); err == nil {
			t.Fatal("CreateCollection: want the server's error, got nil")
		}
	})
}
