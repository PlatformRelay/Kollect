//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportBigQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, emulatorHost, err := startBigQueryEmulator(ctx)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}
		t.Fatalf("start bigquery emulator: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	t.Setenv("BIGQUERY_EMULATOR_HOST", emulatorHost)

	client, err := newEmulatorClient(ctx, "test-project", emulatorHost)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := waitForBigQueryEmulator(ctx, client); err != nil {
		t.Fatalf("wait for emulator: %v", err)
	}

	if err := createDatasetWithRetry(ctx, client, "inventory", &bigquery.DatasetMetadata{}); err != nil {
		t.Fatalf("create dataset: %v", err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: "test-cluster",
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "test-project",
			Dataset: "inventory",
			Table:   "inventory_items",
		},
	}

	backend, err := NewBackend(ctx, spec, nil)
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	t.Cleanup(backend.Close)

	items := []collect.Item{
		{
			TargetNamespace: "apps",
			TargetName:      "web",
			Namespace:       "apps",
			Name:            "demo",
			Version:         "v1",
			Kind:            "Deployment",
			UID:             "uid-1",
			Attributes:      map[string]any{"replicas": 2},
		},
	}
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	count, err := queryCount(ctx, client, "test-project", "inventory", "inventory_items")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}

	updated := items
	updated[0].Attributes = map[string]any{"replicas": 3}
	payload, err = json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export upsert: %v", err)
	}

	replicas, err := queryReplicaCount(ctx, client, "test-project", "inventory", "inventory_items", "uid-1")
	if err != nil {
		t.Fatal(err)
	}
	if replicas != 3 {
		t.Fatalf("replicas = %d, want 3", replicas)
	}

	extra := collect.Item{
		TargetNamespace: "apps",
		TargetName:      "web",
		Namespace:       "apps",
		Name:            "extra",
		Version:         "v1",
		Kind:            "Deployment",
		UID:             "uid-stale",
	}
	extraPayload, err := json.Marshal(append(items, extra))
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, extraPayload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export with extra row: %v", err)
	}

	count, err = queryCount(ctx, client, "test-project", "inventory", "inventory_items")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("row count after extra export = %d, want 2", count)
	}

	reducedPayload, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Export(ctx, reducedPayload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export delete recon: %v", err)
	}

	count, err = queryCount(ctx, client, "test-project", "inventory", "inventory_items")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count after delete recon = %d, want 1", count)
	}

	if err := backend.Export(ctx, []byte("[]"), "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export empty snapshot: %v", err)
	}
	count, err = queryCount(ctx, client, "test-project", "inventory", "inventory_items")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("row count after empty export = %d, want 0", count)
	}
}

func TestBigQueryExistingModeMissingTable(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, emulatorHost, err := startBigQueryEmulator(ctx)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}
		t.Fatalf("start bigquery emulator: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	t.Setenv("BIGQUERY_EMULATOR_HOST", emulatorHost)

	client, err := newEmulatorClient(ctx, "test-project", emulatorHost)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := waitForBigQueryEmulator(ctx, client); err != nil {
		t.Fatalf("wait for emulator: %v", err)
	}
	if err := createDatasetWithRetry(ctx, client, "inventory", &bigquery.DatasetMetadata{}); err != nil {
		t.Fatalf("create dataset: %v", err)
	}

	_, err = NewBackend(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		Provisioning: &kollectdevv1alpha1.ProvisioningSpec{
			Mode: kollectdevv1alpha1.ProvisioningModeExisting,
		},
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "test-project",
			Dataset: "inventory",
			Table:   "missing_table",
		},
	}, nil)
	if err == nil {
		t.Fatal("expected missing table error in existing mode")
	}
}

func TestBigQueryConnectionProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, emulatorHost, err := startBigQueryEmulator(ctx)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}
		t.Fatalf("start bigquery emulator: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	t.Setenv("BIGQUERY_EMULATOR_HOST", emulatorHost)

	client, err := newEmulatorClient(ctx, "test-project", emulatorHost)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := waitForBigQueryEmulator(ctx, client); err != nil {
		t.Fatalf("wait for emulator: %v", err)
	}
	if err := createDatasetWithRetry(ctx, client, "inventory", &bigquery.DatasetMetadata{}); err != nil {
		t.Fatalf("create dataset: %v", err)
	}

	err = TestConnection(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "test-project",
			Dataset: "inventory",
			Table:   "inventory_items",
		},
	}, nil)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}

func startBigQueryEmulator(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/goccy/bigquery-emulator:latest",
		ExposedPorts: []string{"9050/tcp"},
		Cmd:          []string{"--project=test-project"},
		WaitingFor:   wait.ForListeningPort("9050/tcp"),
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
	port, err := container.MappedPort(ctx, "9050/tcp")
	if err != nil {
		return nil, "", err
	}

	return container, net.JoinHostPort(host, port.Port()), nil
}

func queryCount(ctx context.Context, client *bigquery.Client, project, dataset, table string) (int64, error) {
	query := client.Query(fmt.Sprintf("SELECT COUNT(*) AS c FROM `%s.%s.%s`", project, dataset, table))
	it, err := query.Read(ctx)
	if err != nil {
		return 0, err
	}

	var row struct {
		C int64 `bigquery:"c"`
	}
	err = it.Next(&row)
	if err != nil {
		return 0, err
	}

	return row.C, nil
}

func queryReplicaCount(ctx context.Context, client *bigquery.Client, project, dataset, table, uid string) (int64, error) {
	query := client.Query(fmt.Sprintf(
		"SELECT TO_JSON_STRING(payload) AS payload_json FROM `%s.%s.%s` WHERE source_uid = @uid",
		project,
		dataset,
		table,
	))
	query.Parameters = []bigquery.QueryParameter{{Name: "uid", Value: uid}}
	it, err := query.Read(ctx)
	if err != nil {
		return 0, err
	}

	var row struct {
		PayloadJSON string `bigquery:"payload_json"`
	}
	if err := it.Next(&row); err != nil {
		if err == iterator.Done {
			return 0, fmt.Errorf("row %q not found", uid)
		}
		return 0, err
	}

	var payload struct {
		Attributes struct {
			Replicas int64 `json:"replicas"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
		return 0, err
	}

	return payload.Attributes.Replicas, nil
}

func newEmulatorClient(ctx context.Context, project, host string) (*bigquery.Client, error) {
	return bigquery.NewClient(
		ctx,
		project,
		option.WithEndpoint("http://"+host),
		option.WithoutAuthentication(),
	)
}

func waitForBigQueryEmulator(ctx context.Context, client *bigquery.Client) error {
	deadline := time.Now().Add(60 * time.Second)
	probe := client.Dataset("_kollect_probe")
	var lastErr error

	for time.Now().Before(deadline) {
		err := probe.Create(ctx, &bigquery.DatasetMetadata{})
		if err == nil {
			_ = probe.Delete(ctx)

			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			_ = probe.Delete(ctx)

			return nil
		}

		lastErr = err
		time.Sleep(time.Second)
	}

	return fmt.Errorf("bigquery emulator not ready: %w", lastErr)
}

func createDatasetWithRetry(ctx context.Context, client *bigquery.Client, datasetID string, md *bigquery.DatasetMetadata) error {
	deadline := time.Now().Add(60 * time.Second)
	ds := client.Dataset(datasetID)
	var lastErr error

	for time.Now().Before(deadline) {
		err := ds.Create(ctx, md)
		if err == nil {
			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}

		lastErr = err
		time.Sleep(time.Second)
	}

	return fmt.Errorf("create dataset %q: %w", datasetID, lastErr)
}
