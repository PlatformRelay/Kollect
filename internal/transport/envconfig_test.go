// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"testing"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "")
	t.Setenv("KOLLECT_HUB_STREAM", "")
	t.Setenv("KOLLECT_HUB_GROUP", "")

	cfg := ConfigFromEnv()
	if cfg.Type != TypeInProcess {
		t.Fatalf("type = %q", cfg.Type)
	}

	if cfg.Stream != defaultStream {
		t.Fatalf("stream = %q", cfg.Stream)
	}

	if cfg.Group != "kollect-hub" {
		t.Fatalf("group = %q", cfg.Group)
	}
}

func TestConfigFromEnvKafka(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "kafka")
	t.Setenv("KOLLECT_KAFKA_BROKERS", "b1:9092, b2:9092")
	t.Setenv("KOLLECT_KAFKA_TOPIC", "reports")

	cfg := ConfigFromEnv()
	if cfg.Type != TypeKafka {
		t.Fatalf("type = %q", cfg.Type)
	}

	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "b1:9092" {
		t.Fatalf("brokers = %v", cfg.Kafka.Brokers)
	}

	if cfg.Kafka.Topic != "reports" {
		t.Fatalf("topic = %q", cfg.Kafka.Topic)
	}
}

func TestConfigFromEnvNATS(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "nats")
	t.Setenv("KOLLECT_NATS_URL", "nats://127.0.0.1:4222")

	cfg := ConfigFromEnv()
	if cfg.Type != TypeNATS {
		t.Fatalf("type = %q", cfg.Type)
	}

	if cfg.NATS.URL != "nats://127.0.0.1:4222" {
		t.Fatalf("url = %q", cfg.NATS.URL)
	}
}

func TestConfigFromEnvHTTP(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "http")
	t.Setenv("KOLLECT_HUB_URL", "https://hub.example/hub")

	cfg := ConfigFromEnv()
	if cfg.Type != TypeHTTP || cfg.HTTP.URL != "https://hub.example/hub" {
		t.Fatalf("http cfg = %#v", cfg)
	}
}

func TestConfigFromEnvRedisTLS(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "redis")
	t.Setenv("KOLLECT_REDIS_URL", "rediss://localhost:6379")
	t.Setenv("KOLLECT_TRANSPORT_TLS_INSECURE_SKIP_VERIFY", "true")

	cfg := ConfigFromEnv()
	if !cfg.Redis.TLS.InsecureSkipVerify {
		t.Fatal("expected redis tls insecure skip verify")
	}
}
