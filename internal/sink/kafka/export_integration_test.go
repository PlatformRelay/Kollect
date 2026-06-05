//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"

	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportKafka(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, err := redpanda.Run(ctx, "docker.redpanda.com/redpandadata/redpanda:v24.2.4")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start redpanda: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	broker, err := container.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatal(err)
	}

	const topic = "inventory-events"
	if err := createTopic(ctx, broker, topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "kafka",
		Cluster: "test-cluster",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{broker},
			Topic:   topic,
		},
	}

	backend, err := NewBackend(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})

	payload := []byte(`[{"namespace":"apps","uid":"uid-1"}]`)
	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: "kollect-test",
	})
	t.Cleanup(func() {
		_ = reader.Close()
	})

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.SchemaVersion != export.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", envelope.SchemaVersion, export.SchemaVersion)
	}

	if envelope.Cluster != "test-cluster" {
		t.Fatalf("cluster = %q, want test-cluster", envelope.Cluster)
	}

	if envelope.Namespace != "apps" {
		t.Fatalf("namespace = %q, want apps", envelope.Namespace)
	}

	if string(envelope.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", envelope.Payload, payload)
	}
}

func createTopic(ctx context.Context, broker, topic string) error {
	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	return controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
}
