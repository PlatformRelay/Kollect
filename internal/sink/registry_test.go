// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
)

func TestRegistry_NewBackend(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	gitBackend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "git",
		Endpoint: "https://example.com/inventory.git",
	}, BuildContext{})
	if err != nil {
		t.Fatalf("NewBackend(git) error = %v", err)
	}

	if gitBackend.Type() != "git" {
		t.Fatalf("Type() = %q, want git", gitBackend.Type())
	}

	gitlabBackend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "gitlab",
		Endpoint: "https://gitlab.example.com/platform/inventory.git",
	}, BuildContext{})
	if err != nil {
		t.Fatalf("NewBackend(gitlab) error = %v", err)
	}

	if gitlabBackend.Type() != "gitlab" {
		t.Fatalf("Type() = %q, want gitlab", gitlabBackend.Type())
	}

	s3Backend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://inventory-bucket/prefix",
	}, BuildContext{
		SecretData: map[string][]byte{
			"accessKeyID":     []byte("key"),
			"secretAccessKey": []byte("secret"),
		},
	})
	if err != nil {
		t.Fatalf("NewBackend(s3) error = %v", err)
	}

	if s3Backend.Type() != "s3" {
		t.Fatalf("Type() = %q, want s3", s3Backend.Type())
	}

	if _, pgErr := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "items",
		},
	}, BuildContext{
		DatabaseSecretData: map[string][]byte{
			"dsn": []byte("postgres://127.0.0.1:1/inventory?sslmode=disable&connect_timeout=1"),
		},
	}); pgErr == nil {
		t.Fatal("NewBackend(postgres) expected connection error without running postgres")
	}

	kafkaBackend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"localhost:9092"},
			Topic:   "inventory",
		},
	}, BuildContext{})
	if err != nil {
		t.Fatalf("NewBackend(kafka) error = %v", err)
	}

	if kafkaBackend.Type() != "kafka" {
		t.Fatalf("Type() = %q, want kafka", kafkaBackend.Type())
	}

	natsBackend, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     "nats://localhost:4222",
			Subject: "inventory.events",
		},
	}, BuildContext{})
	if err != nil {
		t.Fatalf("NewBackend(nats) error = %v", err)
	}

	if natsBackend.Type() != "nats" {
		t.Fatalf("Type() = %q, want nats", natsBackend.Type())
	}

	if _, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}, BuildContext{}); err == nil {
		t.Fatal("NewBackend(unknown) expected error")
	}

	_ = context.Background()
	_ = gitBackend
	_ = gitlabBackend
	_ = s3Backend
}

func TestRegistry_NewBackend_unknownTypeIsTerminal(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	// Former ADR-0414 stub types must fail terminally, not retry forever (EC-P1-04).
	for _, sinkType := range []string{"unknown", "http", "azureblob", "bigquery"} {
		_, err := reg.NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: sinkType}, BuildContext{})
		if err == nil {
			t.Fatalf("NewBackend(%s) expected error", sinkType)
		}

		if !kollecterrors.IsTerminal(err) {
			t.Fatalf("NewBackend(%s) error class = %s, want terminal: %v", sinkType, kollecterrors.ClassOf(err), err)
		}

		if !strings.Contains(err.Error(), sinkType) {
			t.Fatalf("NewBackend(%s) error should name the offending type: %v", sinkType, err)
		}

		if !strings.Contains(err.Error(), "supported:") || !strings.Contains(err.Error(), "git") {
			t.Fatalf("NewBackend(%s) error should list supported types: %v", sinkType, err)
		}
	}
}
