// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestTestConnection_missingTopic(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type:  "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"127.0.0.1:1"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error when topic is missing")
	}
}

func TestTestConnection_withSASLCredentials(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"127.0.0.1:1"},
			Topic:   "inventory",
		},
	}, map[string][]byte{
		"username": []byte("user"),
		"password": []byte("pass"),
	})
	if err == nil {
		t.Fatal("expected dial error for unreachable broker")
	}
}

func TestTestConnection_missingBrokers(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type:  "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{Topic: "inventory"},
	}, nil)
	if err == nil {
		t.Fatal("expected error when brokers are missing")
	}
}

func TestTestConnection_unreachableBroker(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"127.0.0.1:1"},
			Topic:   "inventory",
		},
	}, nil)
	if err == nil {
		t.Fatal("expected dial error for unreachable broker")
	}
}
