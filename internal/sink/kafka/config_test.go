// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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

func TestConfigFromSpec_validationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec kollectdevv1alpha1.KollectSinkSpec
		want string
	}{
		{
			name: "wrong type",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "nats"},
			want: "expected kafka sink",
		},
		{
			name: "missing topic",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:  "kafka",
				Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"broker:9092"}},
			},
			want: "topic",
		},
		{
			name: "blank brokers only",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:  "kafka",
				Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"  "}, Topic: "inventory"},
			},
			want: "at least one broker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ConfigFromSpec(tt.spec, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ConfigFromSpec() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestConfigFromSpec_tokenOverridesPassword(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"broker:9092"},
			Topic:   "inventory",
		},
	}, map[string][]byte{
		"password": []byte("pass"),
		"token":    []byte("token"),
	})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}
	if cfg.Password != "token" {
		t.Fatalf("password = %q, want token override", cfg.Password)
	}
}
