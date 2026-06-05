// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"errors"
	"testing"
)

func TestNewKafkaTransport_validConfig(t *testing.T) {
	t.Parallel()

	pub, sub, err := NewTransport(Config{
		Type: TypeKafka,
		Kafka: KafkaConfig{
			Brokers: []string{"127.0.0.1:9092"},
			Topic:   "reports",
			Group:   "test-group",
		},
	})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	if pub == nil || sub == nil {
		t.Fatal("expected publisher and subscriber")
	}

	kt, ok := pub.(*KafkaTransport)
	if !ok {
		t.Fatalf("publisher type = %T", pub)
	}
	if kt.topic != "reports" || kt.group != "test-group" {
		t.Fatalf("transport = topic %q group %q", kt.topic, kt.group)
	}
	if err := Close(kt); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewKafkaTransport_defaultTopicAndGroup(t *testing.T) {
	t.Parallel()

	pub, _, err := NewTransport(Config{
		Type: TypeKafka,
		Kafka: KafkaConfig{
			Brokers: []string{"broker:9092"},
			Group:   "hub-workers",
		},
	})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	kt := pub.(*KafkaTransport)
	if kt.topic != defaultKafkaTopic {
		t.Fatalf("topic = %q, want default", kt.topic)
	}
	if kt.group != "hub-workers" {
		t.Fatalf("group = %q", kt.group)
	}
	_ = Close(kt)
}

func TestKafkaTransport_subscribeDuplicateSubject(t *testing.T) {
	t.Parallel()

	pub, _, err := NewTransport(Config{
		Type:  TypeKafka,
		Kafka: KafkaConfig{Brokers: []string{"127.0.0.1:9092"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	kt := pub.(*KafkaTransport)
	t.Cleanup(func() { _ = Close(kt) })

	nopHandler := func(_ context.Context, _ []byte) error { return nil }
	if err := kt.Subscribe(t.Context(), "inventory/default", nopHandler); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}
	if err := kt.Subscribe(t.Context(), "inventory/default", nopHandler); err == nil {
		t.Fatal("expected duplicate subscribe error")
	}
}

func TestSplitCommaEnv_empty(t *testing.T) {
	t.Setenv("KOLLECT_KAFKA_BROKERS", "  , , ")
	if got := splitCommaEnv("KOLLECT_KAFKA_BROKERS"); len(got) != 0 {
		t.Fatalf("splitCommaEnv = %v, want empty", got)
	}
}

func TestEnvOr_fallback(t *testing.T) {
	t.Setenv("KOLLECT_TEST_FALLBACK", "")
	if got := envOr("KOLLECT_TEST_FALLBACK", "fallback"); got != "fallback" {
		t.Fatalf("envOr = %q", got)
	}
}

func TestKafkaHandlerErrorPreventsCommit(t *testing.T) {
	t.Parallel()

	handlerErr := errors.New("merge failed")
	committed := false
	commit := func() { committed = true }

	if err := kafkaConsumeHandler(t.Context(), []byte(`{"ok":true}`), func(_ context.Context, _ []byte) error {
		return handlerErr
	}, commit); err == nil {
		t.Fatal("expected handler error")
	}
	if committed {
		t.Fatal("handler error must not commit Kafka offset")
	}
}

func TestKafkaHandlerSuccessCommits(t *testing.T) {
	t.Parallel()

	committed := false
	commit := func() { committed = true }

	if err := kafkaConsumeHandler(t.Context(), []byte(`{"ok":true}`), func(_ context.Context, _ []byte) error {
		return nil
	}, commit); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !committed {
		t.Fatal("successful handler must commit Kafka offset")
	}
}
