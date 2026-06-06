//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/integrationtest"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/transport"
)

func TestHubKafkaSpokeToConsumerMergeExport(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("kollect"),
		postgres.WithPassword("kollect"),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(context.Background()) })

	pgConn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForPostgres(ctx, pgConn); err != nil {
		t.Fatal(err)
	}

	kafkaContainer, err := redpanda.Run(ctx, "docker.redpanda.com/redpandadata/redpanda:v24.2.4")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start redpanda: %v", err)
	}
	t.Cleanup(func() { _ = kafkaContainer.Terminate(context.Background()) })

	broker, err := kafkaContainer.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatal(err)
	}

	const topic = "kollect.hub.reports"
	if err := createKafkaTopic(ctx, broker, topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}

	pub, sub, err := transport.NewTransport(transport.Config{
		Type: transport.TypeKafka,
		Kafka: transport.KafkaConfig{
			Brokers: []string{broker},
			Topic:   topic,
			Group:   "kollect-hub-spoke-test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = transport.Close(pub)
		_ = transport.Close(sub)
	})

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
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "platform"},
		Data:       map[string][]byte{"dsn": []byte(pgConn)},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pgSink, pgSecret).
		Build()

	store := collect.NewStore()
	merger := hub.NewMerger(store)
	exporter := &hub.Exporter{
		Store:    store,
		Client:   cl,
		Registry: sink.NewRegistry(),
		Config: hub.ExportConfig{
			ExportNamespace: "platform",
			SinkRefs:        []string{"hub-postgres"},
		},
	}

	consumer := hub.NewConsumer(sub, merger, "inventory/reports", "kafka-hub", nil, hub.ConsumerOptions{
		Exporter: exporter,
	})

	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()

	consumerErr := make(chan error, 1)
	go func() {
		consumerErr <- consumer.Start(consumerCtx)
	}()

	// Allow Kafka consumer group join before publish (avoids CI flake).
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	report := hub.SpokeReport{
		APIVersion:    export.WireAPIVersion,
		SchemaVersion: export.SchemaVersion,
		Cluster:       "spoke-a",
		InventoryRef: hub.InventoryRef{
			Namespace: "team-a",
			Name:      "team-inventory",
		},
		Items: []collect.Item{
			{
				Namespace:  "apps",
				Name:       "web",
				UID:        "uid-kafka-roundtrip",
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

	if err := pub.Publish(ctx, "inventory/reports", payload); err != nil {
		t.Fatalf("spoke publish: %v", err)
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if store.TotalCount() == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if store.TotalCount() != 1 {
		t.Fatalf("hub store count = %d, want 1 after Kafka consume", store.TotalCount())
	}

	pool, err := pgxpool.New(ctx, pgConn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	for time.Now().Before(deadline) {
		var count int
		err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM public.inventory_items
WHERE inventory_namespace = $1 AND inventory_name = $2 AND source_uid = $3
`, "team-a", "team-inventory", "uid-kafka-roundtrip").Scan(&count)
		if err == nil && count == 1 {
			consumerCancel()
			select {
			case err := <-consumerErr:
				if err != nil && err != context.Canceled {
					t.Fatalf("consumer exit: %v", err)
				}
			case <-time.After(5 * time.Second):
			}

			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("postgres export not visible after Kafka spoke→hub round-trip within %s", 45*time.Second)
}
