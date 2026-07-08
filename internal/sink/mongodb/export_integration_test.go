//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportMongoDB(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	const (
		database   = "inventory"
		collection = "items"
		cluster    = "test-cluster"
		objectPath = "inventory/apps/inv.json"
	)

	spec := defaultMongoSpec(database, collection, cluster)
	backend, err := NewBackend(ctx, spec, map[string][]byte{"uri": []byte(uri)})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	t.Cleanup(backend.Close)

	coll := backend.client.Database(database).Collection(collection)
	scopeFilter := bson.M{
		"inventory_namespace": "apps",
		"inventory_name":      "inv",
		"cluster":             cluster,
	}

	items := []collect.Item{
		{
			TargetNamespace: "apps",
			TargetName:      "deployments",
			Namespace:       "apps",
			Name:            "api",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 2},
		},
		{
			TargetNamespace: "apps",
			TargetName:      "deployments",
			Namespace:       "apps",
			Name:            "worker",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-2",
			Attributes:      map[string]any{"replicas": 1},
		},
	}

	payload, err := marshalExport(items, cluster, 1)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	count, err := countDocuments(ctx, coll, scopeFilter)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("document count = %d, want 2", count)
	}

	// Upsert again with updated attributes — row count stays stable, payload changes.
	updated := append([]collect.Item(nil), items...)
	updated[0].Attributes = map[string]any{"replicas": 3}
	updatedPayload, err := marshalExport(updated, cluster, 2)
	if err != nil {
		t.Fatalf("marshal updated envelope: %v", err)
	}

	if err := backend.Export(ctx, updatedPayload, objectPath); err != nil {
		t.Fatalf("Export upsert: %v", err)
	}

	count, err = countDocuments(ctx, coll, scopeFilter)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("document count after upsert = %d, want 2", count)
	}

	replicas, err := documentAttribute(ctx, coll, scopeFilter, "uid-1", "replicas")
	if err != nil {
		t.Fatal(err)
	}
	if replicas != float64(3) {
		t.Fatalf("replicas = %v, want 3", replicas)
	}

	// Reduced snapshot deletes stale documents (ADR-0401).
	reduced := []collect.Item{items[0]}
	reducedPayload, err := marshalExport(reduced, cluster, 3)
	if err != nil {
		t.Fatalf("marshal reduced envelope: %v", err)
	}

	if err := backend.Export(ctx, reducedPayload, objectPath); err != nil {
		t.Fatalf("Export reduced: %v", err)
	}

	count, err = countDocuments(ctx, coll, scopeFilter)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("document count after delete recon = %d, want 1", count)
	}

	// Empty snapshot deletes all documents for this inventory scope.
	emptyPayload, err := marshalExport(nil, cluster, 4)
	if err != nil {
		t.Fatalf("marshal empty envelope: %v", err)
	}

	if err := backend.Export(ctx, emptyPayload, objectPath); err != nil {
		t.Fatalf("Export empty snapshot: %v", err)
	}

	count, err = countDocuments(ctx, coll, scopeFilter)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("document count after empty export = %d, want 0", count)
	}
}

func TestExportMongoDB_scopeIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	const database = "inventory"
	spec := defaultMongoSpec(database, "scope-items", "test-cluster")
	backend, err := NewBackend(ctx, spec, map[string][]byte{"uri": []byte(uri)})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	t.Cleanup(backend.Close)

	item := []collect.Item{{
		TargetName: "deployments",
		Namespace:  "apps",
		Name:       "api",
		UID:        "uid-a",
	}}

	payloadA, err := marshalExport(item, "test-cluster", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, payloadA, "inventory/apps/inv-a.json"); err != nil {
		t.Fatalf("Export scope A: %v", err)
	}

	payloadB, err := marshalExport(item, "test-cluster", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, payloadB, "inventory/apps/inv-b.json"); err != nil {
		t.Fatalf("Export scope B: %v", err)
	}

	coll := backend.client.Database(database).Collection("scope-items")
	total, err := countDocuments(ctx, coll, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total documents = %d, want 2 (one per inventory scope)", total)
	}

	emptyPayload, err := marshalExport(nil, "test-cluster", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, emptyPayload, "inventory/apps/inv-a.json"); err != nil {
		t.Fatalf("Export empty scope A: %v", err)
	}

	remaining, err := countDocuments(ctx, coll, bson.M{"inventory_name": "inv-b"})
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("scope B documents = %d, want 1 after scope A purge", remaining)
	}
}

func TestNewBackend_ensureCollectionIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	spec := defaultMongoSpec("inventory", "idempotent-items", "test-cluster")
	secret := map[string][]byte{"uri": []byte(uri)}

	first, err := NewBackend(ctx, spec, secret)
	if err != nil {
		t.Fatalf("first NewBackend: %v", err)
	}
	first.Close()

	second, err := NewBackend(ctx, spec, secret)
	if err != nil {
		t.Fatalf("second NewBackend: %v", err)
	}
	second.Close()
}

func TestExportMongoDB_existingProvisioningMode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// provisioning.mode=existing but collection does not exist yet → must error
	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: "test-cluster",
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "missing-collection",
		},
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
	}

	_, err = NewBackend(ctx, spec, map[string][]byte{"uri": []byte(uri)})
	if err == nil {
		t.Fatal("expected error when collection does not exist in provisioning.mode=existing")
	}
}

func TestExportMongoDB_existingProvisioningModeSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	const (
		database   = "inventory"
		collection = "preexisting"
	)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	if err := client.Database(database).CreateCollection(ctx, collection); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: "test-cluster",
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    database,
			Collection:  collection,
		},
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{Mode: kollectdevv1alpha1.ProvisioningModeExisting},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"uri": []byte(uri)})
	if err != nil {
		t.Fatalf("NewBackend existing collection: %v", err)
	}
	t.Cleanup(backend.Close)

	items := []collect.Item{{
		TargetName: "pods",
		Namespace:  "apps",
		Name:       "demo",
		UID:        "uid-existing",
	}}
	payload, err := marshalExport(items, "test-cluster", 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, payload, "inventory/apps/preexisting.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	count, err := countDocuments(ctx, client.Database(database).Collection(collection), bson.M{
		"inventory_namespace": "apps",
		"inventory_name":      "preexisting",
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("document count = %d, want 1", count)
	}
}

func TestConnectionMongoDB(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}

	if err := TestConnection(ctx, spec, map[string][]byte{"uri": []byte(uri)}); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}

func defaultMongoSpec(database, collection, cluster string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: cluster,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    database,
			Collection:  collection,
		},
	}
}

func marshalExport(items []collect.Item, cluster string, generation int64) ([]byte, error) {
	return collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: generation,
		Cluster:    cluster,
		ExportedAt: time.Now().UTC(),
	})
}

func countDocuments(ctx context.Context, coll *mongo.Collection, filter bson.M) (int64, error) {
	return coll.CountDocuments(ctx, filter)
}

func documentAttribute(
	ctx context.Context,
	coll *mongo.Collection,
	scopeFilter bson.M,
	sourceUID string,
	attr string,
) (float64, error) {
	filter := bson.M{}
	for k, v := range scopeFilter {
		filter[k] = v
	}
	filter["source_uid"] = sourceUID

	var doc struct {
		Payload map[string]any `bson:"payload"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return 0, err
	}

	attrs, ok := doc.Payload["attributes"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("payload attributes missing for uid %s", sourceUID)
	}

	value, ok := attrs[attr].(float64)
	if !ok {
		return 0, fmt.Errorf("attribute %q type = %T, want float64", attr, attrs[attr])
	}

	return value, nil
}

func startMongoContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:7",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", err
	}

	port, err := container.MappedPort(ctx, "27017/tcp")
	if err != nil {
		return nil, "", err
	}

	uri := fmt.Sprintf("mongodb://%s:%s", host, port.Port())

	return container, uri, nil
}

func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}

	if integrationtest.IsDockerUnavailable(err) {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "docker provider") ||
		strings.Contains(msg, "rootless docker")
}
