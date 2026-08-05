// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/netguard"
)

const connectTimeout = 30 * time.Second

// Backend upserts inventory documents into MongoDB.
type Backend struct {
	cfg    Config
	client *mongo.Client
	coll   exportCollection
	admin  collectionAdmin
}

// collectionAdmin abstracts list/create/index for unit tests (AUD26-TEST-02b).
type collectionAdmin interface {
	ListCollectionNames(ctx context.Context, filter interface{}) ([]string, error)
	CreateCollection(ctx context.Context, name string) error
	EnsureUniqueIndex(ctx context.Context, keys bson.D) error
}

type mongoCollectionAdmin struct {
	db   *mongo.Database
	coll *mongo.Collection
}

func (a mongoCollectionAdmin) ListCollectionNames(ctx context.Context, filter interface{}) ([]string, error) {
	return a.db.ListCollectionNames(ctx, filter)
}

func (a mongoCollectionAdmin) CreateCollection(ctx context.Context, name string) error {
	return a.db.CreateCollection(ctx, name)
}

func (a mongoCollectionAdmin) EnsureUniqueIndex(ctx context.Context, keys bson.D) error {
	index := mongo.IndexModel{
		Keys:    keys,
		Options: options.Index().SetUnique(true),
	}
	_, err := a.coll.Indexes().CreateOne(ctx, index)
	return err
}

type deleteManyCollection interface {
	DeleteMany(context.Context, interface{}, ...*options.DeleteOptions) (*mongo.DeleteResult, error)
}

type exportCollection interface {
	ReplaceOne(context.Context, interface{}, interface{}, ...*options.ReplaceOptions) (*mongo.UpdateResult, error)
	DeleteMany(context.Context, interface{}, ...*options.DeleteOptions) (*mongo.DeleteResult, error)
}

// NewBackend constructs a MongoDB sink backend.
func NewBackend(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return nil, err
	}

	connectCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, connectTimeout)
		defer cancel()
	}

	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(cfg.URI).SetDialer(netguard.DefaultDialer))
	if err != nil {
		return nil, fmt.Errorf("mongodb connect: %w", err)
	}

	if err := client.Ping(connectCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())

		return nil, fmt.Errorf("mongodb ping: %w", err)
	}

	db := client.Database(cfg.Database)
	coll := db.Collection(cfg.Collection)
	b := &Backend{
		cfg:    cfg,
		client: client,
		coll:   coll,
		admin:  mongoCollectionAdmin{db: db, coll: coll},
	}

	if cfg.ProvisioningMode == kollectdevv1alpha1.ProvisioningModeExisting {
		if err := b.verifyCollection(connectCtx); err != nil {
			_ = client.Disconnect(context.Background())

			return nil, err
		}
	} else if err := b.ensureCollection(connectCtx); err != nil {
		_ = client.Disconnect(context.Background())

		return nil, err
	}

	return b, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Capabilities reports document upsert with delete reconciliation (ADR-0417).
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.RelationalStore()
}

// Close releases the MongoDB client.
func (b *Backend) Close() {
	if b.client != nil {
		_ = b.client.Disconnect(context.Background())
	}
}

// Export upserts each inventory item and deletes stale documents (ADR-0401).
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	items, err := collect.ItemsFromExportPayload(payload)
	if err != nil {
		return fmt.Errorf("mongodb export: decode payload: %w", err)
	}

	scope := newExportScope(objectPath, b.cfg.Cluster)
	exportedAt := time.Now().UTC()

	for _, item := range items {
		doc, err := itemDocument(scope, item, exportedAt)
		if err != nil {
			return err
		}

		if _, err := b.coll.ReplaceOne(ctx, upsertFilter(scope, item), doc, options.Replace().SetUpsert(true)); err != nil {
			return classifyError(fmt.Errorf("%w: %w", ErrUpsertFailed, err))
		}
	}

	return deleteStaleDocuments(ctx, b.coll, scope, items)
}

func (b *Backend) verifyCollection(ctx context.Context) error {
	names, err := b.admin.ListCollectionNames(ctx, bson.M{"name": b.cfg.Collection})
	if err != nil {
		return fmt.Errorf("mongodb verify collection: %w", err)
	}

	for _, name := range names {
		if name == b.cfg.Collection {
			return nil
		}
	}

	return fmt.Errorf("mongodb collection %s.%s does not exist (provisioning.mode=existing)",
		b.cfg.Database, b.cfg.Collection)
}

func (b *Backend) ensureCollection(ctx context.Context) error {
	names, err := b.admin.ListCollectionNames(ctx, bson.M{"name": b.cfg.Collection})
	if err != nil {
		return fmt.Errorf("mongodb list collections: %w", err)
	}

	found := false
	for _, name := range names {
		if name == b.cfg.Collection {
			found = true
			break
		}
	}

	if !found {
		if err := b.admin.CreateCollection(ctx, b.cfg.Collection); err != nil {
			return fmt.Errorf("mongodb create collection: %w", err)
		}
	}

	keys := bson.D{
		{Key: "inventory_namespace", Value: 1},
		{Key: "inventory_name", Value: 1},
		{Key: "target_name", Value: 1},
		{Key: "source_uid", Value: 1},
	}
	if err := b.admin.EnsureUniqueIndex(ctx, keys); err != nil {
		return fmt.Errorf("mongodb ensure index: %w", err)
	}

	return nil
}

func itemDocument(scope exportScope, item collect.Item, exportedAt time.Time) (bson.M, error) {
	itemJSON, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("mongodb export: marshal item: %w", err)
	}

	resourceNS := item.Namespace
	if resourceNS == "" {
		resourceNS = scope.inventoryNamespace
	}

	var payload any
	if err := json.Unmarshal(itemJSON, &payload); err != nil {
		return nil, fmt.Errorf("mongodb export: decode item payload: %w", err)
	}

	return bson.M{
		"inventory_namespace": scope.inventoryNamespace,
		"inventory_name":      scope.inventoryName,
		"target_name":         item.TargetName,
		"source_uid":          item.UID,
		"cluster":             scope.cluster,
		"resource_namespace":  resourceNS,
		"payload":             payload,
		"exported_at":         exportedAt,
	}, nil
}

func deleteStaleDocuments(
	ctx context.Context,
	coll deleteManyCollection,
	scope exportScope,
	items []collect.Item,
) error {
	filter, deleteAll := staleDeleteFilter(scope, items)
	_, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		if deleteAll {
			return fmt.Errorf("mongodb delete all: %w", err)
		}

		return fmt.Errorf("mongodb delete stale: %w", err)
	}

	return nil
}

func staleDeleteFilter(scope exportScope, items []collect.Item) (bson.M, bool) {
	filter := scope.filter()
	if len(items) == 0 {
		return filter, true
	}

	orFilters := make([]bson.M, 0, len(items))
	for _, item := range items {
		orFilters = append(orFilters, bson.M{
			"target_name": item.TargetName,
			"source_uid":  item.UID,
		})
	}
	filter["$nor"] = orFilters

	return filter, false
}
