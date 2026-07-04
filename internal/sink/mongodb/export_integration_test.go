//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

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
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start mongodb: %v", err)
	}

	t.Cleanup(func() { _ = container.Terminate(ctx) })

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: "test-cluster",
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}

	backend, err := NewBackend(ctx, spec, map[string][]byte{"uri": []byte(uri)})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	t.Cleanup(backend.Close)

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

	payload, err := collect.MarshalExportEnvelope(items, collect.ExportMetadata{
		Generation: 1,
		Cluster:    "test-cluster",
		ExportedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	const objectPath = "inventory/apps/inv.json"
	if err := backend.Export(ctx, payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Upsert again (idempotent) with one item removed — stale doc must be deleted.
	reduced := []collect.Item{items[0]}
	reducedPayload, err := collect.MarshalExportEnvelope(reduced, collect.ExportMetadata{
		Generation: 2,
		Cluster:    "test-cluster",
		ExportedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal reduced envelope: %v", err)
	}

	if err := backend.Export(ctx, reducedPayload, objectPath); err != nil {
		t.Fatalf("Export reduced: %v", err)
	}
}

func TestExportMongoDB_existingProvisioningMode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
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

func TestConnectionMongoDB(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, uri, err := startMongoContainer(ctx)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
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
