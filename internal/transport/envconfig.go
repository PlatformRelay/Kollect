// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"os"
	"strings"
)

// ConfigFromEnv builds transport settings from standard KOLLECT_* environment variables.
func ConfigFromEnv() Config {
	transportType := os.Getenv("KOLLECT_TRANSPORT_TYPE")
	if transportType == "" {
		transportType = string(TypeInProcess)
	}

	cfg := Config{
		Type:   Type(transportType),
		Stream: envOr("KOLLECT_HUB_STREAM", defaultStream),
		Group:  envOr("KOLLECT_HUB_GROUP", defaultHubGroup),
		ACL:    ACLSettingsFromEnv(),
	}

	tls := TLSSettingsFromEnv()

	switch cfg.Type {
	case TypeHTTP:
		cfg.HTTP.URL = os.Getenv("KOLLECT_HUB_URL")
	case TypeRedis:
		cfg.Redis.URL = os.Getenv("KOLLECT_REDIS_URL")
		cfg.Redis.TLS = tls
	case TypeKafka:
		cfg.Kafka.Brokers = splitCommaEnv("KOLLECT_KAFKA_BROKERS")
		cfg.Kafka.Topic = envOr("KOLLECT_KAFKA_TOPIC", defaultKafkaTopic)
		cfg.Kafka.Group = envOr("KOLLECT_KAFKA_GROUP", cfg.Group)
	case TypeNATS:
		cfg.NATS.URL = os.Getenv("KOLLECT_NATS_URL")
		cfg.NATS.TLS = tls
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func splitCommaEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}

	return out
}
