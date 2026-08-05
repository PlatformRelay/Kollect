// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestMongoCollectionAdmin_listCreateIndex(t *testing.T) {
	t.Parallel()

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list names", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateCursorResponse(1, "db.$cmd.listCollections", mtest.FirstBatch, bson.D{{Key: "name", Value: "items"}}),
		)
		admin := mongoCollectionAdmin{db: mt.Client.Database("db"), coll: mt.Client.Database("db").Collection("items")}
		names, err := admin.ListCollectionNames(context.Background(), bson.M{"name": "items"})
		if err != nil {
			mt.Fatalf("ListCollectionNames: %v", err)
		}
		if len(names) != 1 {
			mt.Fatalf("names = %v", names)
		}
	})

	mt.Run("create collection", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		admin := mongoCollectionAdmin{db: mt.Client.Database("db"), coll: mt.Client.Database("db").Collection("items")}
		if err := admin.CreateCollection(context.Background(), "items"); err != nil {
			mt.Fatalf("CreateCollection: %v", err)
		}
	})

	mt.Run("ensure index", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		admin := mongoCollectionAdmin{db: mt.Client.Database("db"), coll: mt.Coll}
		if err := admin.EnsureUniqueIndex(context.Background(), bson.D{{Key: "source_uid", Value: 1}}); err != nil {
			mt.Fatalf("EnsureUniqueIndex: %v", err)
		}
	})
}
