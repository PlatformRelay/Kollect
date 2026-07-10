// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/secretkv"
)

// TypeName is the KollectSink.spec.type value for Kafka sinks.
const TypeName = "kafka"

const typeName = TypeName

// Config holds resolved Kafka sink settings.
type Config struct {
	Brokers  []string
	Topic    string
	Cluster  string
	Username string
	Password string
}

// ConfigFromSpec validates spec and optional secret data for a kafka sink.
func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
) (Config, error) {
	if spec.Type != typeName {
		return Config{}, fmt.Errorf("expected kafka sink, got %q", spec.Type)
	}

	if spec.Kafka == nil {
		return Config{}, fmt.Errorf("kafka sink requires spec.kafka")
	}

	k := spec.Kafka
	if len(k.Brokers) == 0 {
		return Config{}, fmt.Errorf("kafka sink requires spec.kafka.brokers")
	}

	topic := strings.TrimSpace(k.Topic)
	if topic == "" {
		return Config{}, fmt.Errorf("kafka sink requires spec.kafka.topic")
	}

	brokers := make([]string, 0, len(k.Brokers))
	for _, b := range k.Brokers {
		b = strings.TrimSpace(b)
		if b != "" {
			brokers = append(brokers, b)
		}
	}

	if len(brokers) == 0 {
		return Config{}, fmt.Errorf("kafka sink requires at least one broker")
	}

	cfg := Config{
		Brokers: brokers,
		Topic:   topic,
		Cluster: strings.TrimSpace(spec.Cluster),
	}

	secretkv.AssignIfPresent(secretData, "username", &cfg.Username)

	// Kafka has no separate token field: a present "token" key overrides
	// "password" (token last wins), matching the pre-extraction loop.
	for _, key := range []string{"password", "token"} {
		secretkv.AssignIfPresent(secretData, key, &cfg.Password)
	}

	return cfg, nil
}
