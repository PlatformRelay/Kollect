//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/sink"
	kafkasink "github.com/konih/kollect/internal/sink/kafka"
)

func TestHubExportPostgresAndKafkaParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("kollect"),
		postgres.WithPassword("kollect"),
	)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	pgConn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForPostgres(ctx, pgConn); err != nil {
		t.Fatal(err)
	}

	kafkaContainer, err := redpanda.Run(ctx, "docker.redpanda.com/redpandadata/redpanda:v24.2.4")
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start redpanda: %v", err)
	}
	t.Cleanup(func() { _ = kafkaContainer.Terminate(ctx) })

	broker, err := kafkaContainer.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatal(err)
	}

	const topic = "hub-inventory-events"
	if err := createKafkaTopic(ctx, broker, topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pgSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "hub-postgres", Namespace: "platform"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:    "postgres",
			Cluster: "hub",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
				Schema:      "public",
			},
		},
	}
	kafkaSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "hub-kafka", Namespace: "platform"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:    "kafka",
			Cluster: "hub",
			Kafka: &kollectdevv1alpha1.KafkaSpec{
				Brokers: []string{broker},
				Topic:   topic,
			},
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "platform"},
		Data:       map[string][]byte{"dsn": []byte(pgConn)},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pgSink, kafkaSink, pgSecret).
		Build()

	store := collect.NewStore()
	merger := hub.NewMerger(store)
	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: hub.InventoryRef{
			Namespace: "team-a",
			Name:      "team-inventory",
		},
		Items: []collect.Item{
			{
				Namespace:  "apps",
				Name:       "web",
				UID:        "uid-web",
				Version:    "v1",
				Kind:       "Deployment",
				Attributes: map[string]any{"image": "nginx:1.27"},
			},
		},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := hub.ReceiveReport("spoke-a", payload, merger, []string{"spoke-a"}, true); err != nil {
		t.Fatalf("ReceiveReport: %v", err)
	}

	exporter := &hub.Exporter{
		Store:    store,
		Client:   cl,
		Registry: sink.NewRegistry(),
		Config: hub.ExportConfig{
			ExportNamespace: "platform",
			SinkRefs:        []string{"hub-postgres", "hub-kafka"},
		},
	}

	if err := exporter.ExportAfterMerge(ctx, report); err != nil {
		t.Fatalf("ExportAfterMerge: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgConn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND source_uid = $3
`, "team-a", "team-inventory", "uid-web").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("postgres row count = %d, want 1", count)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: "kollect-hub-test",
	})
	t.Cleanup(func() { _ = reader.Close() })

	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var envelope kafkasink.EventEnvelope
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.Cluster != "hub" {
		t.Fatalf("kafka cluster = %q, want hub", envelope.Cluster)
	}

	if envelope.Namespace != "team-a" {
		t.Fatalf("kafka namespace = %q, want team-a", envelope.Namespace)
	}
}

func waitForPostgres(ctx context.Context, connStr string) error {
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)

			continue
		}

		lastErr = pool.Ping(ctx)
		pool.Close()

		if lastErr == nil {
			return nil
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("postgres not ready: %w", lastErr)
}

func createKafkaTopic(ctx context.Context, broker, topic string) error {
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

func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "permission denied")
}
