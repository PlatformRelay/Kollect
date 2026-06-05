// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpec(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "kafka"}, nil)
	if err == nil {
		t.Fatal("expected error without kafka spec")
	}

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:    "kafka",
		Cluster: "prod-a",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"broker:9092"},
			Topic:   "inventory",
		},
	}, map[string][]byte{"username": []byte("user"), "password": []byte("pass")})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}

	if cfg.Topic != "inventory" {
		t.Fatalf("topic = %q, want inventory", cfg.Topic)
	}

	if cfg.Username != "user" || cfg.Password != "pass" {
		t.Fatalf("SASL creds not resolved")
	}
}
